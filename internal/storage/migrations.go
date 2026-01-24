package storage

import (
	"database/sql"
	"fmt"
)

// migrations holds all database migrations in order
var migrations = []struct {
	version int
	name    string
	sql     string
}{
	{
		version: 1,
		name:    "create_credentials_table",
		sql: `
			CREATE TABLE IF NOT EXISTS credentials (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				username TEXT NOT NULL,
				password_encrypted BLOB NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);
		`,
	},
	{
		version: 2,
		name:    "create_thermostat_state_table",
		sql: `
			CREATE TABLE IF NOT EXISTS thermostat_state (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				device_id INTEGER UNIQUE,
				name TEXT,
				current_temp REAL,
				heat_setpoint REAL,
				cool_setpoint REAL,
				system_mode INTEGER,
				humidity INTEGER,
				is_heating BOOLEAN,
				is_cooling BOOLEAN,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);
			CREATE INDEX IF NOT EXISTS idx_thermostat_device_id ON thermostat_state(device_id);
		`,
	},
	{
		version: 3,
		name:    "create_event_log_table",
		sql: `
			CREATE TABLE IF NOT EXISTS event_log (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
				source TEXT NOT NULL,
				event_type TEXT NOT NULL,
				message TEXT,
				details JSON
			);
			CREATE INDEX IF NOT EXISTS idx_event_log_timestamp ON event_log(timestamp);
			CREATE INDEX IF NOT EXISTS idx_event_log_source ON event_log(source);
			CREATE INDEX IF NOT EXISTS idx_event_log_type ON event_log(event_type);
		`,
	},
	{
		version: 4,
		name:    "create_matter_state_table",
		sql: `
			CREATE TABLE IF NOT EXISTS matter_state (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				is_commissioned BOOLEAN DEFAULT FALSE,
				fabric_id TEXT,
				node_id TEXT,
				qr_code TEXT,
				manual_pair_code TEXT,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);
			INSERT OR IGNORE INTO matter_state (id) VALUES (1);
		`,
	},
	{
		version: 5,
		name:    "create_migrations_table",
		sql: `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				name TEXT NOT NULL,
				applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);
		`,
	},
}

// RunMigrations applies all pending migrations
func RunMigrations(db *sql.DB) error {
	// Ensure migrations table exists
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get current version
	var currentVersion int
	row := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	if err := row.Scan(&currentVersion); err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	// Apply pending migrations
	for _, m := range migrations {
		if m.version <= currentVersion {
			continue
		}

		// Skip the migrations table creation since we already did it
		if m.name == "create_migrations_table" {
			_, err := db.Exec("INSERT INTO schema_migrations (version, name) VALUES (?, ?)", m.version, m.name)
			if err != nil {
				return fmt.Errorf("failed to record migration %d: %w", m.version, err)
			}
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", m.version, err)
		}

		_, err = tx.Exec(m.sql)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %d (%s): %w", m.version, m.name, err)
		}

		_, err = tx.Exec("INSERT INTO schema_migrations (version, name) VALUES (?, ?)", m.version, m.name)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %d: %w", m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", m.version, err)
		}

		fmt.Printf("Applied migration %d: %s\n", m.version, m.name)
	}

	return nil
}

// GetMigrationVersion returns the current schema version
func GetMigrationVersion(db *sql.DB) (int, error) {
	var version int
	row := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	if err := row.Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}
