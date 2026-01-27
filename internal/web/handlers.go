package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gregjohnson/mitsubishi/internal/log"
	"github.com/gregjohnson/mitsubishi/internal/storage"
)

// Version information, set via ldflags at build time
var (
	Version   = "dev"
	BuildDate = "unknown"
)

// StatusResponse represents the overall system status
type StatusResponse struct {
	TCC        ConnectionStatus `json:"tcc"`
	Matter     MatterStatus     `json:"matter"`
	Configured bool             `json:"configured"`
}

// ConnectionStatus represents a connection status
type ConnectionStatus struct {
	Connected bool      `json:"connected"`
	LastPoll  time.Time `json:"last_poll,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// MatterStatus represents Matter bridge status
type MatterStatus struct {
	Running      bool   `json:"running"`
	Commissioned bool   `json:"commissioned"`
	FabricID     string `json:"fabric_id,omitempty"`
}

// ThermostatResponse represents thermostat data for the API
type ThermostatResponse struct {
	DeviceID     int     `json:"device_id"`
	Name         string  `json:"name"`
	CurrentTemp  float64 `json:"current_temp"`
	HeatSetpoint float64 `json:"heat_setpoint"`
	CoolSetpoint float64 `json:"cool_setpoint"`
	SystemMode   string  `json:"system_mode"`
	Humidity     int     `json:"humidity"`
	IsHeating    bool    `json:"is_heating"`
	IsCooling    bool    `json:"is_cooling"`
	UpdatedAt    string  `json:"updated_at"`
}

// ConfigResponse represents configuration status
type ConfigResponse struct {
	HasCredentials bool   `json:"has_credentials"`
	Username       string `json:"username,omitempty"`
}

// CredentialsRequest represents a credentials save request
type CredentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// SetpointRequest represents a setpoint change request
type SetpointRequest struct {
	DeviceID int     `json:"device_id"`
	Type     string  `json:"type"` // "heat" or "cool"
	Value    float64 `json:"value"`
}

// ModeRequest represents a mode change request
type ModeRequest struct {
	DeviceID int    `json:"device_id"`
	Mode     string `json:"mode"`
}

// PairingResponse represents Matter pairing info
type PairingResponse struct {
	QRCode         string `json:"qr_code"`
	ManualPairCode string `json:"manual_pair_code"`
	Commissioned   bool   `json:"commissioned"`
}

// VersionResponse represents version info
type VersionResponse struct {
	Version   string `json:"version"`
	BuildDate string `json:"build_date"`
}

// handleStatus returns overall system status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	tccClient := s.service.GetTCCClient()
	matterBridge := s.service.GetMatterBridge()
	db := s.service.GetDB()

	// Check if credentials are configured
	creds, _ := db.GetCredentials()
	configured := creds != nil

	status := StatusResponse{
		TCC: ConnectionStatus{
			Connected: tccClient.IsAuthenticated(),
		},
		Matter: MatterStatus{
			Running: matterBridge.IsRunning(),
		},
		Configured: configured,
	}

	// Get Matter status if running
	if matterBridge.IsRunning() {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if matterStatus, err := matterBridge.GetStatus(ctx); err == nil {
			status.Matter.Commissioned = matterStatus.Commissioned
			status.Matter.FabricID = matterStatus.FabricID
		}
	}

	writeJSON(w, status)
}

// handleGetThermostat returns thermostat data
func (s *Server) handleGetThermostat(w http.ResponseWriter, r *http.Request) {
	db := s.service.GetDB()

	states, err := db.GetAllThermostatStates()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get thermostat states")
		return
	}

	response := make([]ThermostatResponse, 0, len(states))
	for _, state := range states {
		response = append(response, ThermostatResponse{
			DeviceID:     state.DeviceID,
			Name:         state.Name,
			CurrentTemp:  state.CurrentTemp,
			HeatSetpoint: state.HeatSetpoint,
			CoolSetpoint: state.CoolSetpoint,
			SystemMode:   state.SystemMode.String(),
			Humidity:     state.Humidity,
			IsHeating:    state.IsHeating,
			IsCooling:    state.IsCooling,
			UpdatedAt:    state.UpdatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, response)
}

// handleSetSetpoint changes the thermostat setpoint
func (s *Server) handleSetSetpoint(w http.ResponseWriter, r *http.Request) {
	var req SetpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	db := s.service.GetDB()
	tccClient := s.service.GetTCCClient()
	ctx := r.Context()

	// Get current state for logging
	oldState, _ := db.GetThermostatStateByDeviceID(req.DeviceID)
	oldValue := 0.0
	if oldState != nil {
		if req.Type == "heat" {
			oldValue = oldState.HeatSetpoint
		} else {
			oldValue = oldState.CoolSetpoint
		}
	}

	// Set the setpoint in TCC
	var err error
	switch req.Type {
	case "heat":
		err = tccClient.SetHeatSetpoint(ctx, req.DeviceID, req.Value)
	case "cool":
		err = tccClient.SetCoolSetpoint(ctx, req.DeviceID, req.Value)
	default:
		writeError(w, http.StatusBadRequest, "Invalid setpoint type")
		return
	}

	if err != nil {
		log.Error("Failed to set setpoint: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to set setpoint")
		return
	}

	// Fetch updated state from TCC
	updatedDevice, err := tccClient.GetDeviceData(ctx, req.DeviceID)
	if err != nil {
		log.Warn("Failed to fetch updated state after setpoint change: %v", err)
	} else {
		// Save to database
		state := &storage.ThermostatState{
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
		db.SaveThermostatState(state)

		// Update Matter bridge
		matterBridge := s.service.GetMatterBridge()
		if err := matterBridge.UpdateState(ctx, *updatedDevice); err != nil {
			log.Debug("Failed to update Matter state: %v", err)
		}

		// Broadcast update via WebSocket
		s.hub.Broadcast(map[string]interface{}{
			"type": "thermostat_update",
			"data": ThermostatResponse{
				DeviceID:     updatedDevice.DeviceID,
				Name:         updatedDevice.Name,
				CurrentTemp:  updatedDevice.CurrentTemp,
				HeatSetpoint: updatedDevice.HeatSetpoint,
				CoolSetpoint: updatedDevice.CoolSetpoint,
				SystemMode:   updatedDevice.SystemMode,
				Humidity:     updatedDevice.Humidity,
				IsHeating:    updatedDevice.IsHeating,
				IsCooling:    updatedDevice.IsCooling,
				UpdatedAt:    updatedDevice.UpdatedAt.Format(time.RFC3339),
			},
		})
	}

	// Log the event with details
	db.LogEvent(storage.EventSourceUser, storage.EventTypeTempChange,
		fmt.Sprintf("%s setpoint changed from %.1f°F to %.1f°F",
			strings.Title(req.Type), oldValue, req.Value), map[string]interface{}{
			"device_id":  req.DeviceID,
			"type":       req.Type,
			"old_value":  oldValue,
			"new_value":  req.Value,
		})

	writeJSON(w, map[string]string{"status": "ok"})
}

// handleSetMode changes the thermostat mode
func (s *Server) handleSetMode(w http.ResponseWriter, r *http.Request) {
	var req ModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	db := s.service.GetDB()
	tccClient := s.service.GetTCCClient()
	ctx := r.Context()

	// Get current state for logging
	oldState, _ := db.GetThermostatStateByDeviceID(req.DeviceID)
	oldMode := "unknown"
	if oldState != nil {
		oldMode = oldState.SystemMode.String()
	}

	// Set the mode in TCC
	if err := tccClient.SetSystemMode(ctx, req.DeviceID, req.Mode); err != nil {
		log.Error("Failed to set mode: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to set mode")
		return
	}

	// Fetch updated state from TCC
	updatedDevice, err := tccClient.GetDeviceData(ctx, req.DeviceID)
	if err != nil {
		log.Warn("Failed to fetch updated state after mode change: %v", err)
	} else {
		// Save to database
		state := &storage.ThermostatState{
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
		db.SaveThermostatState(state)

		// Update Matter bridge
		matterBridge := s.service.GetMatterBridge()
		if err := matterBridge.UpdateState(ctx, *updatedDevice); err != nil {
			log.Debug("Failed to update Matter state: %v", err)
		}

		// Broadcast update via WebSocket
		s.hub.Broadcast(map[string]interface{}{
			"type": "thermostat_update",
			"data": ThermostatResponse{
				DeviceID:     updatedDevice.DeviceID,
				Name:         updatedDevice.Name,
				CurrentTemp:  updatedDevice.CurrentTemp,
				HeatSetpoint: updatedDevice.HeatSetpoint,
				CoolSetpoint: updatedDevice.CoolSetpoint,
				SystemMode:   updatedDevice.SystemMode,
				Humidity:     updatedDevice.Humidity,
				IsHeating:    updatedDevice.IsHeating,
				IsCooling:    updatedDevice.IsCooling,
				UpdatedAt:    updatedDevice.UpdatedAt.Format(time.RFC3339),
			},
		})
	}

	// Log the event with details
	db.LogEvent(storage.EventSourceUser, storage.EventTypeModeChange,
		fmt.Sprintf("Mode changed from %s to %s", oldMode, req.Mode), map[string]interface{}{
			"device_id": req.DeviceID,
			"old_mode":  oldMode,
			"new_mode":  req.Mode,
		})

	writeJSON(w, map[string]string{"status": "ok"})
}

// handleGetConfig returns configuration status
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	db := s.service.GetDB()

	creds, err := db.GetCredentials()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get config")
		return
	}

	response := ConfigResponse{
		HasCredentials: creds != nil,
	}
	if creds != nil {
		response.Username = creds.Username
	}

	writeJSON(w, response)
}

// handleSaveCredentials saves TCC credentials
func (s *Server) handleSaveCredentials(w http.ResponseWriter, r *http.Request) {
	var req CredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "Username and password required")
		return
	}

	// Encrypt password
	encKey := s.service.GetEncryptionKey()
	encryptedPassword, err := encKey.EncryptString(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to encrypt password")
		return
	}

	// Save to database
	db := s.service.GetDB()
	if err := db.SaveCredentials(req.Username, encryptedPassword); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save credentials")
		return
	}

	// Update TCC client
	tccClient := s.service.GetTCCClient()
	tccClient.SetCredentials(req.Username, req.Password)

	// Log the event
	db.LogEvent(storage.EventSourceUser, storage.EventTypeCredentials,
		"Credentials saved", map[string]interface{}{"username": req.Username})

	writeJSON(w, map[string]string{"status": "ok"})
}

// handleTestCredentials tests TCC credentials
func (s *Server) handleTestCredentials(w http.ResponseWriter, r *http.Request) {
	var req CredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	db := s.service.GetDB()

	// Log the test attempt
	db.LogEvent(storage.EventSourceUser, storage.EventTypeConnection,
		"Testing TCC connection", map[string]interface{}{"username": req.Username})

	tccClient := s.service.GetTCCClient()
	tccClient.SetCredentials(req.Username, req.Password)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := tccClient.TestConnection(ctx); err != nil {
		// Log failure
		db.LogEvent(storage.EventSourceTCC, storage.EventTypeError,
			"Connection test failed", map[string]interface{}{"error": err.Error()})

		writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Log success
	db.LogEvent(storage.EventSourceTCC, storage.EventTypeConnection,
		"Connection test successful", map[string]interface{}{"username": req.Username})

	writeJSON(w, map[string]interface{}{
		"success": true,
	})
}

// handleGetPairing returns Matter pairing information
func (s *Server) handleGetPairing(w http.ResponseWriter, r *http.Request) {
	matterBridge := s.service.GetMatterBridge()

	// Return a response even if Matter bridge isn't running
	response := PairingResponse{
		Commissioned: false,
	}

	if !matterBridge.IsRunning() {
		// Return empty pairing info with running=false indicator
		writeJSON(w, response)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	info, err := matterBridge.GetPairingInfo(ctx)
	if err != nil {
		log.Debug("Failed to get pairing info: %v", err)
		// Still return a response, just without pairing codes
		writeJSON(w, response)
		return
	}

	status, _ := matterBridge.GetStatus(ctx)

	response.QRCode = info.QRCode
	response.ManualPairCode = info.ManualPairCode
	if status != nil {
		response.Commissioned = status.Commissioned
	}

	writeJSON(w, response)
}

// handleDecommission decommissions the Matter device
func (s *Server) handleDecommission(w http.ResponseWriter, r *http.Request) {
	matterBridge := s.service.GetMatterBridge()
	db := s.service.GetDB()

	if !matterBridge.IsRunning() {
		writeError(w, http.StatusServiceUnavailable, "Matter bridge is not running")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := matterBridge.Decommission(ctx); err != nil {
		log.Error("Failed to decommission device: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to decommission device")
		return
	}

	// Log the event
	db.LogEvent(storage.EventSourceUser, storage.EventTypeConnection,
		"Matter device decommissioned - ready for re-pairing", nil)

	// Broadcast WebSocket event
	s.hub.Broadcast(map[string]interface{}{
		"type": "matter_decommissioned",
	})

	writeJSON(w, map[string]string{"status": "ok"})
}

// handleGetLogs returns event logs
func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	db := s.service.GetDB()

	filter := storage.EventLogFilter{
		Limit: 100,
	}

	// Parse query parameters
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}
	if source := r.URL.Query().Get("source"); source != "" {
		src := storage.EventSource(source)
		filter.Source = &src
	}

	logs, err := db.GetEventLogs(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get logs")
		return
	}

	writeJSON(w, logs)
}

// handleVersion returns version information
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, VersionResponse{
		Version:   Version,
		BuildDate: BuildDate,
	})
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
