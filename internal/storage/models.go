package storage

import (
	"encoding/json"
	"time"
)

// Credentials stores encrypted TCC login credentials
type Credentials struct {
	ID                int       `json:"id"`
	Username          string    `json:"username"`
	PasswordEncrypted []byte    `json:"-"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// SystemMode represents thermostat operating mode
type SystemMode int

const (
	SystemModeOff        SystemMode = 0
	SystemModeHeat       SystemMode = 1
	SystemModeCool       SystemMode = 2
	SystemModeAuto       SystemMode = 3
	SystemModeEmergency  SystemMode = 4
)

func (m SystemMode) String() string {
	switch m {
	case SystemModeOff:
		return "off"
	case SystemModeHeat:
		return "heat"
	case SystemModeCool:
		return "cool"
	case SystemModeAuto:
		return "auto"
	case SystemModeEmergency:
		return "emergency"
	default:
		return "unknown"
	}
}

// ParseSystemMode converts a string to SystemMode
func ParseSystemMode(s string) SystemMode {
	switch s {
	case "off":
		return SystemModeOff
	case "heat":
		return SystemModeHeat
	case "cool":
		return SystemModeCool
	case "auto":
		return SystemModeAuto
	case "emergency":
		return SystemModeEmergency
	default:
		return SystemModeOff
	}
}

// ThermostatState represents the current state of a thermostat
type ThermostatState struct {
	ID            int        `json:"id"`
	DeviceID      int        `json:"device_id"`
	Name          string     `json:"name"`
	CurrentTemp   float64    `json:"current_temp"`
	HeatSetpoint  float64    `json:"heat_setpoint"`
	CoolSetpoint  float64    `json:"cool_setpoint"`
	SystemMode    SystemMode `json:"system_mode"`
	Humidity      int        `json:"humidity"`
	IsHeating     bool       `json:"is_heating"`
	IsCooling     bool       `json:"is_cooling"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// EventSource represents the source of an event
type EventSource string

const (
	EventSourceTCC     EventSource = "tcc"
	EventSourceMatter  EventSource = "matter"
	EventSourceHomeKit EventSource = "homekit"
	EventSourceUser    EventSource = "user"
	EventSourceSystem  EventSource = "system"
)

// EventType represents the type of event
type EventType string

const (
	EventTypeTempChange    EventType = "temp_change"
	EventTypeModeChange    EventType = "mode_change"
	EventTypeConnection    EventType = "connection"
	EventTypeCredentials   EventType = "credentials"
	EventTypeCommissioning EventType = "commissioning"
	EventTypeError         EventType = "error"
	EventTypeInfo          EventType = "info"
	EventTypeStateChange   EventType = "state_change"
)

// EventLog represents a log entry
type EventLog struct {
	ID        int             `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Source    EventSource     `json:"source"`
	EventType EventType       `json:"event_type"`
	Message   string          `json:"message"`
	Details   json.RawMessage `json:"details,omitempty"`
}

// EventLogFilter for querying events
type EventLogFilter struct {
	Source    *EventSource
	EventType *EventType
	Since     *time.Time
	Until     *time.Time
	Limit     int
	Offset    int
}

// MatterState stores Matter commissioning state
type MatterState struct {
	ID              int       `json:"id"`
	IsCommissioned  bool      `json:"is_commissioned"`
	FabricID        string    `json:"fabric_id,omitempty"`
	NodeID          string    `json:"node_id,omitempty"`
	QRCode          string    `json:"qr_code,omitempty"`
	ManualPairCode  string    `json:"manual_pair_code,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
}
