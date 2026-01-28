package matter

import "time"

// ThermostatState represents thermostat state to send to Matter bridge
type ThermostatState struct {
	DeviceID     int     `json:"deviceId"`
	Name         string  `json:"name"`
	CurrentTemp  float64 `json:"currentTemp"`
	HeatSetpoint float64 `json:"heatSetpoint"`
	CoolSetpoint float64 `json:"coolSetpoint"`
	SystemMode   string  `json:"systemMode"`
	Humidity     int     `json:"humidity"`
	IsHeating    bool    `json:"isHeating"`
	IsCooling    bool    `json:"isCooling"`
}

// Command represents a command from HomeKit via Matter
type Command struct {
	Type   string      `json:"type"`
	Action string      `json:"action"`
	Value  interface{} `json:"value"`
}

// StatusResponse represents the Matter bridge status
type StatusResponse struct {
	Running        bool      `json:"running"`
	Commissioned   bool      `json:"commissioned"`
	FabricID       string    `json:"fabric_id,omitempty"`
	NodeID         string    `json:"node_id,omitempty"`
	ConnectedPeers int       `json:"connected_peers"`
	Uptime         int64     `json:"uptime"`
	LastUpdate     time.Time `json:"last_update"`
}

// PairingInfo represents Matter pairing information
type PairingInfo struct {
	QRCode         string `json:"qr_code"`
	ManualPairCode string `json:"manual_pair_code"`
	SetupURL       string `json:"setup_url,omitempty"`
}

// Event represents an event from the Matter bridge
type Event struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// EventType constants
const (
	EventTypeCommand      = "command"
	EventTypeCommissioned = "commissioned"
	EventTypeConnection   = "connection"
	EventTypeError        = "error"
	EventTypeMatterEvent  = "matter_event"
)
