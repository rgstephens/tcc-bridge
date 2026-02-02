package matter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stephens/tcc-bridge/internal/log"
	"github.com/stephens/tcc-bridge/internal/tcc"
)

// Bridge manages communication with the Matter.js service
type Bridge struct {
	baseURL    string
	bridgeDir  string
	process    *Process
	wsConn     *websocket.Conn
	wsMu       sync.Mutex
	httpClient *http.Client
	eventChan  chan Event
	cmdHandler CommandHandler
}

// CommandHandler handles commands from HomeKit
type CommandHandler func(cmd Command) error

// NewBridge creates a new Matter bridge client
func NewBridge(baseURL, bridgeDir string) *Bridge {
	return &Bridge{
		baseURL:   baseURL,
		bridgeDir: bridgeDir,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		eventChan: make(chan Event, 100),
	}
}

// Start starts the Matter bridge process and connects
func (b *Bridge) Start(ctx context.Context) error {
	// Start the Node.js process
	b.process = NewProcess(b.bridgeDir)
	if err := b.process.Start(ctx); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Wait for service to be ready
	if err := b.waitForReady(ctx); err != nil {
		b.process.Stop()
		return fmt.Errorf("service not ready: %w", err)
	}

	// Connect WebSocket for events
	go b.connectWebSocket(ctx)

	return nil
}

// Stop stops the Matter bridge
func (b *Bridge) Stop() {
	b.wsMu.Lock()
	if b.wsConn != nil {
		b.wsConn.Close()
	}
	b.wsMu.Unlock()

	if b.process != nil {
		b.process.Stop()
	}
}

// SetCommandHandler sets the handler for incoming commands
func (b *Bridge) SetCommandHandler(handler CommandHandler) {
	b.cmdHandler = handler
}

// Events returns the event channel
func (b *Bridge) Events() <-chan Event {
	return b.eventChan
}

// GetStatus retrieves the current status
func (b *Bridge) GetStatus(ctx context.Context) (*StatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", b.baseURL+"/status", nil)
	if err != nil {
		return nil, err
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var status StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

// GetPairingInfo retrieves pairing information
func (b *Bridge) GetPairingInfo(ctx context.Context) (*PairingInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", b.baseURL+"/pairing", nil)
	if err != nil {
		return nil, err
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var info PairingInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}

// fahrenheitToCelsius converts Fahrenheit to Celsius
func fahrenheitToCelsius(f float64) float64 {
	return (f - 32) * 5 / 9
}

// UpdateState sends updated thermostat state to the Matter bridge
func (b *Bridge) UpdateState(ctx context.Context, state tcc.ThermostatState) error {
	// Convert temperatures from Fahrenheit (TCC) to Celsius (Matter)
	matterState := ThermostatState{
		DeviceID:     state.DeviceID,
		Name:         state.Name,
		CurrentTemp:  fahrenheitToCelsius(state.CurrentTemp),
		HeatSetpoint: fahrenheitToCelsius(state.HeatSetpoint),
		CoolSetpoint: fahrenheitToCelsius(state.CoolSetpoint),
		SystemMode:   state.SystemMode,
		Humidity:     state.Humidity,
		IsHeating:    state.IsHeating,
		IsCooling:    state.IsCooling,
	}

	log.Debug("Sending to Matter bridge: temp=%.1f°F (%.1f°C), heat=%.1f°F (%.1f°C), cool=%.1f°F (%.1f°C), mode=%s",
		state.CurrentTemp, matterState.CurrentTemp,
		state.HeatSetpoint, matterState.HeatSetpoint,
		state.CoolSetpoint, matterState.CoolSetpoint,
		state.SystemMode)

	jsonData, err := json.Marshal(matterState)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/state", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

// Decommission decommissions the Matter device (factory reset)
func (b *Bridge) Decommission(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", b.baseURL+"/pairing", nil)
	if err != nil {
		return err
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

// IsRunning returns true if the bridge is running
func (b *Bridge) IsRunning() bool {
	if b.process == nil {
		return false
	}
	return b.process.IsRunning()
}

// waitForReady waits for the service to be ready
func (b *Bridge) waitForReady(ctx context.Context) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for service")
		case <-ticker.C:
			status, err := b.GetStatus(ctx)
			if err == nil && status.Running {
				return nil
			}
		}
	}
}

// connectWebSocket connects to the WebSocket endpoint for events
func (b *Bridge) connectWebSocket(ctx context.Context) {
	wsURL := "ws" + b.baseURL[4:] + "/events"

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		b.wsMu.Lock()
		b.wsConn = conn
		b.wsMu.Unlock()

		b.readWebSocket(ctx, conn)

		b.wsMu.Lock()
		b.wsConn = nil
		b.wsMu.Unlock()

		// Reconnect delay
		time.Sleep(time.Second)
	}
}

// readWebSocket reads events from the WebSocket
func (b *Bridge) readWebSocket(ctx context.Context, conn *websocket.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var event Event
		if err := json.Unmarshal(message, &event); err != nil {
			continue
		}

		// Handle commands
		if event.Type == EventTypeCommand && b.cmdHandler != nil {
			var cmd Command
			if cmdData, err := json.Marshal(event.Data); err == nil {
				if json.Unmarshal(cmdData, &cmd) == nil {
					b.cmdHandler(cmd)
				}
			}
		}

		// Send to event channel
		select {
		case b.eventChan <- event:
		default:
			// Channel full, drop event
		}
	}
}
