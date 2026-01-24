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

	log.Info("Starting TCC-Matter Bridge")

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

func (s *Service) pollTCC(ctx context.Context) {
	if !s.tccClient.IsAuthenticated() {
		// Try to authenticate
		if err := s.tccClient.Login(ctx); err != nil {
			// Check for rate limiting
			if strings.Contains(err.Error(), "rate_limited") {
				log.Warn("TCC rate limited: %v", err)
				s.db.LogEvent(storage.EventSourceTCC, storage.EventTypeError,
					"Rate limited by TCC API", map[string]interface{}{"error": err.Error()})
			} else {
				log.Debug("TCC login failed: %v", err)
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

		// Push to Matter bridge
		if err := s.matterBridge.UpdateState(ctx, device); err != nil {
			log.Debug("Failed to update Matter state: %v", err)
		}
	}

	log.Debug("Polled %d devices from TCC", len(devices))
}
