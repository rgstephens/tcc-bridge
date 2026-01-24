package tcc

import (
	"encoding/json"
	"time"
)

// DeviceInfo represents a thermostat device from TCC
type DeviceInfo struct {
	DeviceID   int    `json:"DeviceID"`
	DeviceType int    `json:"DeviceType"`
	Name       string `json:"Name"`
	IsAlive    bool   `json:"IsAlive"`
}

// ZoneData represents zone information from TCC
type ZoneData struct {
	DeviceID         int     `json:"DeviceID"`
	Name             string  `json:"Name"`
	CurrentTemp      float64 `json:"DispTemperature"`
	HeatSetpoint     float64 `json:"HeatSetpoint"`
	CoolSetpoint     float64 `json:"CoolSetpoint"`
	IndoorHumidity   float64 `json:"IndoorHumidity"`
	SystemSwitchPos  int     `json:"SystemSwitchPosition"`
	EquipmentStatus  int     `json:"EquipmentOutputStatus"`
	IsFanRunning     bool    `json:"IsFanRunning"`
	CanHeat          bool    `json:"CanHeat"`
	CanCool          bool    `json:"CanCool"`
	TemperatureScale string  `json:"ScheduleCapable"` // This isn't right, need to check actual response
}

// LocationData represents a location from TCC
type LocationData struct {
	LocationID int        `json:"LocationID"`
	Name       string     `json:"Name"`
	Devices    []ZoneData `json:"Zones"`
}

// ZoneListResponse represents the response from GetZoneListData
type ZoneListResponse struct {
	Success   bool           `json:"success"`
	Locations []LocationData `json:"Locations"`
}

// DeviceDataResponse represents the response from CheckDataSession
type DeviceDataResponse struct {
	Success          bool    `json:"success"`
	DeviceID         int     `json:"deviceID"`
	DispTemperature  float64 `json:"latestData>uiData>DispTemperature"`
	HeatSetpoint     float64 `json:"latestData>uiData>HeatSetpoint"`
	CoolSetpoint     float64 `json:"latestData>uiData>CoolSetpoint"`
	IndoorHumidity   int     `json:"latestData>uiData>IndoorHumidity"`
	SystemSwitchPos  int     `json:"latestData>uiData>SystemSwitchPosition"`
	EquipmentStatus  int     `json:"latestData>uiData>EquipmentOutputStatus"`

	// Raw data for parsing
	LatestData json.RawMessage `json:"latestData"`
}

// LatestData represents the nested latestData structure
type LatestData struct {
	UIData UIData `json:"uiData"`
}

// UIData represents the UI data from CheckDataSession
type UIData struct {
	DispTemperature        float64 `json:"DispTemperature"`
	HeatSetpoint           float64 `json:"HeatSetpoint"`
	CoolSetpoint           float64 `json:"CoolSetpoint"`
	IndoorHumidity         float64 `json:"IndoorHumidity"`
	OutdoorHumidity        float64 `json:"OutdoorHumidity"`
	OutdoorTemperature     float64 `json:"OutdoorTemperature"`
	SystemSwitchPosition   int     `json:"SystemSwitchPosition"`
	EquipmentOutputStatus  int     `json:"EquipmentOutputStatus"`
	IsFanRunning           bool    `json:"IsFanRunning"`
	DisplayedUnits         string  `json:"DisplayUnits"` // "F" or "C"
	StatusHeat             int     `json:"StatusHeat"`
	StatusCool             int     `json:"StatusCool"`
	DeviceID               int     `json:"DeviceID"`
}

// ControlRequest represents a request to change thermostat settings
type ControlRequest struct {
	DeviceID               int      `json:"DeviceID"`
	SystemSwitch           *int     `json:"SystemSwitch,omitempty"`
	HeatSetpoint           *float64 `json:"HeatSetpoint,omitempty"`
	CoolSetpoint           *float64 `json:"CoolSetpoint,omitempty"`
	HeatNextPeriod         *int     `json:"HeatNextPeriod,omitempty"`
	CoolNextPeriod         *int     `json:"CoolNextPeriod,omitempty"`
	StatusHeat             *int     `json:"StatusHeat,omitempty"`
	StatusCool             *int     `json:"StatusCool,omitempty"`
	FanMode                *int     `json:"FanMode,omitempty"`
}

// SystemMode constants
const (
	TCCModeEmergencyHeat = 0
	TCCModeHeat          = 1
	TCCModeOff           = 2
	TCCModeCool          = 3
	TCCModeAuto          = 4
)

// EquipmentStatus constants
const (
	EquipmentOff     = 0
	EquipmentHeating = 1
	EquipmentCooling = 2
)

// ThermostatState represents the parsed thermostat state
type ThermostatState struct {
	DeviceID      int       `json:"device_id"`
	Name          string    `json:"name"`
	CurrentTemp   float64   `json:"current_temp"`
	HeatSetpoint  float64   `json:"heat_setpoint"`
	CoolSetpoint  float64   `json:"cool_setpoint"`
	SystemMode    string    `json:"system_mode"`
	Humidity      int       `json:"humidity"`
	IsHeating     bool      `json:"is_heating"`
	IsCooling     bool      `json:"is_cooling"`
	Units         string    `json:"units"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SystemModeFromTCC converts TCC system switch position to mode string
func SystemModeFromTCC(pos int) string {
	switch pos {
	case TCCModeEmergencyHeat:
		return "emergency"
	case TCCModeHeat:
		return "heat"
	case TCCModeOff:
		return "off"
	case TCCModeCool:
		return "cool"
	case TCCModeAuto:
		return "auto"
	default:
		return "unknown"
	}
}

// SystemModeToTCC converts mode string to TCC system switch position
func SystemModeToTCC(mode string) int {
	switch mode {
	case "emergency":
		return TCCModeEmergencyHeat
	case "heat":
		return TCCModeHeat
	case "off":
		return TCCModeOff
	case "cool":
		return TCCModeCool
	case "auto":
		return TCCModeAuto
	default:
		return TCCModeOff
	}
}

// IsEquipmentHeating returns true if equipment is heating
func IsEquipmentHeating(status int) bool {
	return status == EquipmentHeating
}

// IsEquipmentCooling returns true if equipment is cooling
func IsEquipmentCooling(status int) bool {
	return status == EquipmentCooling
}
