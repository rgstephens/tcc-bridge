package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gregjohnson/mitsubishi/internal/config"
	"github.com/gregjohnson/mitsubishi/internal/log"
	"github.com/gregjohnson/mitsubishi/internal/matter"
	"github.com/gregjohnson/mitsubishi/internal/storage"
	"github.com/gregjohnson/mitsubishi/internal/tcc"
	"github.com/gregjohnson/mitsubishi/internal/web"
)

func main() {
	configPath := flag.String("config", "", "Path to configuration file")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Set up logging
	if *debug {
		log.SetDefaultLevel(log.LevelDebug)
	}

	log.Info("Starting TCC-Matter Bridge %s (built %s)", web.Version, web.BuildDate)

	// Load configuration
	var cfg *config.Config
	var err error
	if *configPath != "" {
		cfg, err = config.Load(*configPath)
		if err != nil {
			log.Error("Failed to load config: %v", err)
			os.Exit(1)
		}
	} else {
		cfg = config.DefaultConfig()
	}

	// Ensure data directory exists
	if err := cfg.EnsureDataDir(); err != nil {
		log.Error("Failed to create data directory: %v", err)
		os.Exit(1)
	}

	// Open database
	db, err := storage.Open(cfg.DatabasePath())
	if err != nil {
		log.Error("Failed to open database: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	log.Info("Database initialized at %s", cfg.DatabasePath())

	// Load encryption key
	encKey, err := storage.LoadOrCreateKey(cfg.EncryptionKeyPath)
	if err != nil {
		log.Error("Failed to load encryption key: %v", err)
		os.Exit(1)
	}

	// Create TCC client
	tccClient, err := tcc.NewClient(cfg.TCCBaseURL)
	if err != nil {
		log.Error("Failed to create TCC client: %v", err)
		os.Exit(1)
	}

	// Load stored credentials
	creds, err := db.GetCredentials()
	if err != nil {
		log.Error("Failed to load credentials: %v", err)
		os.Exit(1)
	}
	if creds != nil {
		password, err := encKey.DecryptString(creds.PasswordEncrypted)
		if err != nil {
			log.Warn("Failed to decrypt stored password: %v", err)
		} else {
			tccClient.SetCredentials(creds.Username, password)
			log.Info("Loaded stored credentials for %s", creds.Username)
		}
	}

	// Create Matter bridge
	matterBridge := matter.NewBridge(cfg.MatterBridgeURL, cfg.MatterBridgeDir)

	// Create service
	svc := &Service{
		cfg:          cfg,
		db:           db,
		encKey:       encKey,
		tccClient:    tccClient,
		matterBridge: matterBridge,
	}

	// Create and start web server
	webServer := web.NewServer(cfg.ServerPort, svc)

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info("Shutting down...")
		cancel()
	}()

	// Start Matter bridge subprocess
	if err := matterBridge.Start(ctx); err != nil {
		log.Error("Failed to start Matter bridge: %v", err)
		// Continue anyway - Matter bridge might not be built yet
	}

	// Set up command handler for HomeKit commands
	matterBridge.SetCommandHandler(func(cmd matter.Command) error {
		return svc.handleMatterCommand(ctx, cmd)
	})

	// Start polling loop
	go svc.runPollingLoop(ctx)

	// Start web server
	log.Info("Starting web server on port %d", cfg.ServerPort)
	if err := webServer.Run(ctx); err != nil {
		log.Error("Web server error: %v", err)
	}

	// Clean up
	matterBridge.Stop()
	log.Info("Shutdown complete")
}

// Service orchestrates the bridge components
type Service struct {
	cfg          *config.Config
	db           *storage.DB
	encKey       *storage.EncryptionKey
	tccClient    *tcc.Client
	matterBridge *matter.Bridge
}

// GetDB returns the database
func (s *Service) GetDB() *storage.DB {
	return s.db
}

// GetEncryptionKey returns the encryption key
func (s *Service) GetEncryptionKey() *storage.EncryptionKey {
	return s.encKey
}

// GetTCCClient returns the TCC client
func (s *Service) GetTCCClient() *tcc.Client {
	return s.tccClient
}

// GetMatterBridge returns the Matter bridge
func (s *Service) GetMatterBridge() *matter.Bridge {
	return s.matterBridge
}

// GetConfig returns the configuration
func (s *Service) GetConfig() *config.Config {
	return s.cfg
}

// runPollingLoop polls TCC at regular intervals
func (s *Service) runPollingLoop(ctx context.Context) {
	log.Info("Starting TCC polling loop (interval: %d seconds)", s.cfg.TCCPollInterval)

	// Initial poll
	s.pollTCC(ctx)

	ticker := time.NewTicker(time.Duration(s.cfg.TCCPollInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollTCC(ctx)
		}
	}
}

// handleMatterCommand processes commands from HomeKit via Matter bridge
func (s *Service) handleMatterCommand(ctx context.Context, cmd matter.Command) error {
	log.Debug("Processing HomeKit command: %s = %v", cmd.Action, cmd.Value)

	// Get device ID from database (for now, use first device)
	state, err := s.db.GetThermostatState()
	if err != nil {
		return fmt.Errorf("failed to get thermostat state: %w", err)
	}
	deviceID := state.DeviceID

	// Get old state for logging
	oldState, _ := s.db.GetThermostatStateByDeviceID(deviceID)

	// Process the command
	switch cmd.Action {
	case "setSystemMode":
		mode, ok := cmd.Value.(string)
		if !ok {
			return fmt.Errorf("invalid system mode value type")
		}

		oldMode := "unknown"
		if oldState != nil {
			oldMode = oldState.SystemMode.String()
		}

		// Set mode in TCC
		if err := s.tccClient.SetSystemMode(ctx, deviceID, mode); err != nil {
			log.Error("Failed to set mode from HomeKit: %v", err)
			return err
		}

		// Fetch updated state
		updatedDevice, err := s.tccClient.GetDeviceData(ctx, deviceID)
		if err != nil {
			log.Warn("Failed to fetch updated state after HomeKit mode change: %v", err)
		} else {
			// Save to database
			newState := &storage.ThermostatState{
				DeviceID:     updatedDevice.DeviceID,
				Name:         updatedDevice.Name,
				CurrentTemp:  updatedDevice.CurrentTemp,
				HeatSetpoint: updatedDevice.HeatSetpoint,
				CoolSetpoint: updatedDevice.CoolSetpoint,
				SystemMode:   storage.ParseSystemMode(updatedDevice.SystemMode),
				Humidity:     updatedDevice.Humidity,
				IsHeating:    updatedDevice.IsHeating,
				IsCooling:    updatedDevice.IsCooling,
			}
			s.db.SaveThermostatState(newState)

			// Update Matter bridge
			s.matterBridge.UpdateState(ctx, *updatedDevice)
		}

		// Log the change
		s.db.LogEvent(storage.EventSourceHomeKit, storage.EventTypeModeChange,
			fmt.Sprintf("Mode changed from %s to %s", oldMode, mode),
			map[string]interface{}{
				"device_id": deviceID,
				"old_mode":  oldMode,
				"new_mode":  mode,
			})

		log.Info("HomeKit: Mode changed from %s to %s", oldMode, mode)

	case "setHeatingSetpoint":
		// Value comes in Celsius, need to convert to Fahrenheit
		celsius, ok := cmd.Value.(float64)
		if !ok {
			return fmt.Errorf("invalid setpoint value type")
		}
		fahrenheit := celsius*9/5 + 32

		oldSetpoint := 0.0
		if oldState != nil {
			oldSetpoint = oldState.HeatSetpoint
		}

		// Set heat setpoint in TCC
		if err := s.tccClient.SetHeatSetpoint(ctx, deviceID, fahrenheit); err != nil {
			log.Error("Failed to set heat setpoint from HomeKit: %v", err)
			return err
		}

		// Fetch updated state
		updatedDevice, err := s.tccClient.GetDeviceData(ctx, deviceID)
		if err != nil {
			log.Warn("Failed to fetch updated state after HomeKit setpoint change: %v", err)
		} else {
			// Save to database
			newState := &storage.ThermostatState{
				DeviceID:     updatedDevice.DeviceID,
				Name:         updatedDevice.Name,
				CurrentTemp:  updatedDevice.CurrentTemp,
				HeatSetpoint: updatedDevice.HeatSetpoint,
				CoolSetpoint: updatedDevice.CoolSetpoint,
				SystemMode:   storage.ParseSystemMode(updatedDevice.SystemMode),
				Humidity:     updatedDevice.Humidity,
				IsHeating:    updatedDevice.IsHeating,
				IsCooling:    updatedDevice.IsCooling,
			}
			s.db.SaveThermostatState(newState)

			// Update Matter bridge
			s.matterBridge.UpdateState(ctx, *updatedDevice)
		}

		// Log the change
		s.db.LogEvent(storage.EventSourceHomeKit, storage.EventTypeTempChange,
			fmt.Sprintf("Heat setpoint changed from %.1f°F to %.1f°F", oldSetpoint, fahrenheit),
			map[string]interface{}{
				"device_id":     deviceID,
				"type":          "heat",
				"old_setpoint":  oldSetpoint,
				"new_setpoint":  fahrenheit,
			})

		log.Info("HomeKit: Heat setpoint changed from %.1f°F to %.1f°F", oldSetpoint, fahrenheit)

	case "setCoolingSetpoint":
		// Value comes in Celsius, need to convert to Fahrenheit
		celsius, ok := cmd.Value.(float64)
		if !ok {
			return fmt.Errorf("invalid setpoint value type")
		}
		fahrenheit := celsius*9/5 + 32

		oldSetpoint := 0.0
		if oldState != nil {
			oldSetpoint = oldState.CoolSetpoint
		}

		// Set cool setpoint in TCC
		if err := s.tccClient.SetCoolSetpoint(ctx, deviceID, fahrenheit); err != nil {
			log.Error("Failed to set cool setpoint from HomeKit: %v", err)
			return err
		}

		// Fetch updated state
		updatedDevice, err := s.tccClient.GetDeviceData(ctx, deviceID)
		if err != nil {
			log.Warn("Failed to fetch updated state after HomeKit setpoint change: %v", err)
		} else {
			// Save to database
			newState := &storage.ThermostatState{
				DeviceID:     updatedDevice.DeviceID,
				Name:         updatedDevice.Name,
				CurrentTemp:  updatedDevice.CurrentTemp,
				HeatSetpoint: updatedDevice.HeatSetpoint,
				CoolSetpoint: updatedDevice.CoolSetpoint,
				SystemMode:   storage.ParseSystemMode(updatedDevice.SystemMode),
				Humidity:     updatedDevice.Humidity,
				IsHeating:    updatedDevice.IsHeating,
				IsCooling:    updatedDevice.IsCooling,
			}
			s.db.SaveThermostatState(newState)

			// Update Matter bridge
			s.matterBridge.UpdateState(ctx, *updatedDevice)
		}

		// Log the change
		s.db.LogEvent(storage.EventSourceHomeKit, storage.EventTypeTempChange,
			fmt.Sprintf("Cool setpoint changed from %.1f°F to %.1f°F", oldSetpoint, fahrenheit),
			map[string]interface{}{
				"device_id":     deviceID,
				"type":          "cool",
				"old_setpoint":  oldSetpoint,
				"new_setpoint":  fahrenheit,
			})

		log.Info("HomeKit: Cool setpoint changed from %.1f°F to %.1f°F", oldSetpoint, fahrenheit)

	default:
		log.Warn("Unknown HomeKit command: %s", cmd.Action)
		return fmt.Errorf("unknown command: %s", cmd.Action)
	}

	return nil
}

func (s *Service) pollTCC(ctx context.Context) {
	if !s.tccClient.IsAuthenticated() {
		// Try to authenticate
		if err := s.tccClient.Login(ctx); err != nil {
			// Check for rate limiting
			if strings.Contains(err.Error(), "rate_limited") {
				log.Warn("TCC rate limited: %v", err)
				s.db.LogEvent(storage.EventSourceTCC, storage.EventTypeError,
					"Rate limited by TCC API", map[string]interface{}{"error": err.Error()})
			} else if strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "connection refused") {
				log.Error("TCC connection failed: %v", err)
				s.db.LogEvent(storage.EventSourceTCC, storage.EventTypeError,
					"Connection to TCC failed (timeout or network error)", map[string]interface{}{"error": err.Error()})
			} else {
				log.Warn("TCC login failed: %v", err)
				s.db.LogEvent(storage.EventSourceTCC, storage.EventTypeError,
					fmt.Sprintf("Login failed: %v", err), nil)
			}
			return
		}
	}

	devices, err := s.tccClient.GetDevices(ctx)
	if err != nil {
		// Check for rate limiting
		if strings.Contains(err.Error(), "rate_limited") || strings.Contains(err.Error(), "rate limit") {
			log.Warn("TCC rate limited: %v", err)
			s.db.LogEvent(storage.EventSourceTCC, storage.EventTypeError,
				"Rate limited by TCC API", map[string]interface{}{"error": err.Error()})
		} else {
			log.Error("Failed to poll TCC: %v", err)
			s.db.LogEvent(storage.EventSourceTCC, storage.EventTypeError,
				fmt.Sprintf("Poll failed: %v", err), nil)
		}
		return
	}

	for _, device := range devices {
		// Get previous state to detect changes
		prevState, _ := s.db.GetThermostatStateByDeviceID(device.DeviceID)

		// Check if any values changed
		hasChanges := prevState == nil ||
			prevState.CurrentTemp != device.CurrentTemp ||
			prevState.HeatSetpoint != device.HeatSetpoint ||
			prevState.CoolSetpoint != device.CoolSetpoint ||
			string(prevState.SystemMode) != device.SystemMode ||
			prevState.Humidity != device.Humidity

		// Update database
		state := &storage.ThermostatState{
			DeviceID:     device.DeviceID,
			Name:         device.Name,
			CurrentTemp:  device.CurrentTemp,
			HeatSetpoint: device.HeatSetpoint,
			CoolSetpoint: device.CoolSetpoint,
			SystemMode:   storage.ParseSystemMode(device.SystemMode),
			Humidity:     device.Humidity,
			IsHeating:    device.IsHeating,
			IsCooling:    device.IsCooling,
		}
		if err := s.db.SaveThermostatState(state); err != nil {
			log.Error("Failed to save thermostat state: %v", err)
		}

		// Only log and push to Matter if values changed
		if hasChanges {
			// Log state change from TCC
			s.db.LogEvent(storage.EventSourceTCC, storage.EventTypeStateChange,
				fmt.Sprintf("State changed: temp=%.1f°F, heat=%.1f°F, cool=%.1f°F, mode=%s",
					device.CurrentTemp, device.HeatSetpoint, device.CoolSetpoint, device.SystemMode),
				map[string]interface{}{
					"device_id":     device.DeviceID,
					"current_temp":  device.CurrentTemp,
					"heat_setpoint": device.HeatSetpoint,
					"cool_setpoint": device.CoolSetpoint,
					"system_mode":   device.SystemMode,
					"humidity":      device.Humidity,
				})

			// Push to Matter bridge
			if err := s.matterBridge.UpdateState(ctx, device); err != nil {
				log.Debug("Failed to update Matter state: %v", err)
			} else {
				s.db.LogEvent(storage.EventSourceMatter, storage.EventTypeStateChange,
					fmt.Sprintf("Sent to HomeKit: temp=%.1f°F, heat=%.1f°F, cool=%.1f°F, mode=%s",
						device.CurrentTemp, device.HeatSetpoint, device.CoolSetpoint, device.SystemMode),
					map[string]interface{}{
						"device_id":     device.DeviceID,
						"current_temp":  device.CurrentTemp,
						"heat_setpoint": device.HeatSetpoint,
						"cool_setpoint": device.CoolSetpoint,
						"system_mode":   device.SystemMode,
					})
			}
		}
	}

	log.Debug("Polled %d devices from TCC", len(devices))
}
