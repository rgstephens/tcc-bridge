package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
}

// Open creates a new database connection and runs migrations
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations
	if err := RunMigrations(conn); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// --- Credentials ---

// SaveCredentials stores encrypted TCC credentials
func (db *DB) SaveCredentials(username string, passwordEncrypted []byte) error {
	// Delete existing credentials first (single-user system)
	_, err := db.conn.Exec("DELETE FROM credentials")
	if err != nil {
		return fmt.Errorf("failed to clear credentials: %w", err)
	}

	_, err = db.conn.Exec(
		"INSERT INTO credentials (username, password_encrypted, created_at, updated_at) VALUES (?, ?, ?, ?)",
		username, passwordEncrypted, time.Now(), time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	return nil
}

// GetCredentials retrieves stored credentials
func (db *DB) GetCredentials() (*Credentials, error) {
	row := db.conn.QueryRow(
		"SELECT id, username, password_encrypted, created_at, updated_at FROM credentials LIMIT 1",
	)

	var cred Credentials
	err := row.Scan(&cred.ID, &cred.Username, &cred.PasswordEncrypted, &cred.CreatedAt, &cred.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	return &cred, nil
}

// DeleteCredentials removes stored credentials
func (db *DB) DeleteCredentials() error {
	_, err := db.conn.Exec("DELETE FROM credentials")
	return err
}

// --- Thermostat State ---

// SaveThermostatState saves or updates thermostat state
func (db *DB) SaveThermostatState(state *ThermostatState) error {
	_, err := db.conn.Exec(`
		INSERT INTO thermostat_state (device_id, name, current_temp, heat_setpoint, cool_setpoint, system_mode, humidity, is_heating, is_cooling, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(device_id) DO UPDATE SET
			name = excluded.name,
			current_temp = excluded.current_temp,
			heat_setpoint = excluded.heat_setpoint,
			cool_setpoint = excluded.cool_setpoint,
			system_mode = excluded.system_mode,
			humidity = excluded.humidity,
			is_heating = excluded.is_heating,
			is_cooling = excluded.is_cooling,
			updated_at = excluded.updated_at
	`, state.DeviceID, state.Name, state.CurrentTemp, state.HeatSetpoint, state.CoolSetpoint,
		state.SystemMode, state.Humidity, state.IsHeating, state.IsCooling, time.Now())

	if err != nil {
		return fmt.Errorf("failed to save thermostat state: %w", err)
	}

	return nil
}

// GetThermostatState retrieves the current thermostat state
func (db *DB) GetThermostatState() (*ThermostatState, error) {
	row := db.conn.QueryRow(`
		SELECT id, device_id, name, current_temp, heat_setpoint, cool_setpoint, system_mode, humidity, is_heating, is_cooling, updated_at
		FROM thermostat_state
		LIMIT 1
	`)

	var state ThermostatState
	err := row.Scan(
		&state.ID, &state.DeviceID, &state.Name, &state.CurrentTemp, &state.HeatSetpoint,
		&state.CoolSetpoint, &state.SystemMode, &state.Humidity, &state.IsHeating, &state.IsCooling, &state.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get thermostat state: %w", err)
	}

	return &state, nil
}

// GetAllThermostatStates retrieves all thermostat states
func (db *DB) GetAllThermostatStates() ([]ThermostatState, error) {
	rows, err := db.conn.Query(`
		SELECT id, device_id, name, current_temp, heat_setpoint, cool_setpoint, system_mode, humidity, is_heating, is_cooling, updated_at
		FROM thermostat_state
		ORDER BY device_id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query thermostat states: %w", err)
	}
	defer rows.Close()

	var states []ThermostatState
	for rows.Next() {
		var state ThermostatState
		err := rows.Scan(
			&state.ID, &state.DeviceID, &state.Name, &state.CurrentTemp, &state.HeatSetpoint,
			&state.CoolSetpoint, &state.SystemMode, &state.Humidity, &state.IsHeating, &state.IsCooling, &state.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan thermostat state: %w", err)
		}
		states = append(states, state)
	}

	return states, nil
}

// GetThermostatStateByDeviceID retrieves thermostat state for a specific device
func (db *DB) GetThermostatStateByDeviceID(deviceID int) (*ThermostatState, error) {
	var state ThermostatState
	err := db.conn.QueryRow(`
		SELECT id, device_id, name, current_temp, heat_setpoint, cool_setpoint, system_mode, humidity, is_heating, is_cooling, updated_at
		FROM thermostat_state
		WHERE device_id = ?
		LIMIT 1
	`, deviceID).Scan(
		&state.ID, &state.DeviceID, &state.Name, &state.CurrentTemp, &state.HeatSetpoint,
		&state.CoolSetpoint, &state.SystemMode, &state.Humidity, &state.IsHeating, &state.IsCooling, &state.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get thermostat state for device %d: %w", deviceID, err)
	}
	return &state, nil
}

// --- Event Log ---

// LogEvent records an event in the log
func (db *DB) LogEvent(source EventSource, eventType EventType, message string, details interface{}) error {
	var detailsJSON []byte
	if details != nil {
		var err error
		detailsJSON, err = json.Marshal(details)
		if err != nil {
			return fmt.Errorf("failed to marshal event details: %w", err)
		}
	}

	_, err := db.conn.Exec(
		"INSERT INTO event_log (timestamp, source, event_type, message, details) VALUES (?, ?, ?, ?, ?)",
		time.Now(), source, eventType, message, detailsJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to log event: %w", err)
	}

	return nil
}

// GetEventLogs retrieves events with optional filtering
func (db *DB) GetEventLogs(filter EventLogFilter) ([]EventLog, error) {
	query := "SELECT id, timestamp, source, event_type, message, details FROM event_log WHERE 1=1"
	args := []interface{}{}

	if filter.Source != nil {
		query += " AND source = ?"
		args = append(args, *filter.Source)
	}
	if filter.EventType != nil {
		query += " AND event_type = ?"
		args = append(args, *filter.EventType)
	}
	if filter.Since != nil {
		query += " AND timestamp >= ?"
		args = append(args, *filter.Since)
	}
	if filter.Until != nil {
		query += " AND timestamp <= ?"
		args = append(args, *filter.Until)
	}

	query += " ORDER BY timestamp DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query event logs: %w", err)
	}
	defer rows.Close()

	var logs []EventLog
	for rows.Next() {
		var log EventLog
		var details sql.NullString
		err := rows.Scan(&log.ID, &log.Timestamp, &log.Source, &log.EventType, &log.Message, &details)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event log: %w", err)
		}
		if details.Valid && details.String != "" {
			log.Details = json.RawMessage(details.String)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// --- Matter State ---

// GetMatterState retrieves the Matter commissioning state
func (db *DB) GetMatterState() (*MatterState, error) {
	row := db.conn.QueryRow(`
		SELECT id, is_commissioned, fabric_id, node_id, qr_code, manual_pair_code, updated_at
		FROM matter_state WHERE id = 1
	`)

	var state MatterState
	var fabricID, nodeID, qrCode, manualPairCode sql.NullString
	err := row.Scan(&state.ID, &state.IsCommissioned, &fabricID, &nodeID, &qrCode, &manualPairCode, &state.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get matter state: %w", err)
	}

	state.FabricID = fabricID.String
	state.NodeID = nodeID.String
	state.QRCode = qrCode.String
	state.ManualPairCode = manualPairCode.String

	return &state, nil
}

// SaveMatterState saves the Matter commissioning state
func (db *DB) SaveMatterState(state *MatterState) error {
	_, err := db.conn.Exec(`
		UPDATE matter_state SET
			is_commissioned = ?,
			fabric_id = ?,
			node_id = ?,
			qr_code = ?,
			manual_pair_code = ?,
			updated_at = ?
		WHERE id = 1
	`, state.IsCommissioned, state.FabricID, state.NodeID, state.QRCode, state.ManualPairCode, time.Now())

	if err != nil {
		return fmt.Errorf("failed to save matter state: %w", err)
	}

	return nil
}

// PruneEventLogs removes old event logs
func (db *DB) PruneEventLogs(olderThan time.Time) (int64, error) {
	result, err := db.conn.Exec("DELETE FROM event_log WHERE timestamp < ?", olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to prune event logs: %w", err)
	}

	return result.RowsAffected()
}
