package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds application configuration
type Config struct {
	// Server settings
	ServerPort int    `json:"server_port"`
	DataDir    string `json:"data_dir"`

	// Matter bridge settings
	MatterPort       int    `json:"matter_port"`
	MatterBridgeURL  string `json:"matter_bridge_url"`
	MatterBridgeDir  string `json:"matter_bridge_dir"`

	// TCC settings
	TCCBaseURL      string `json:"tcc_base_url"`
	TCCPollInterval int    `json:"tcc_poll_interval_seconds"`

	// Encryption key path (for TCC credentials)
	EncryptionKeyPath string `json:"encryption_key_path"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".tcc-bridge")

	return &Config{
		ServerPort:        8080,
		DataDir:           dataDir,
		MatterPort:        5540,
		MatterBridgeURL:   "http://localhost:5540",
		MatterBridgeDir:   "./matter-bridge",
		TCCBaseURL:        "https://mytotalconnectcomfort.com",
		TCCPollInterval:   600, // 10 minutes
		EncryptionKeyPath: filepath.Join(dataDir, "encryption.key"),
	}
}

// Load reads configuration from a JSON file
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes configuration to a JSON file
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// EnsureDataDir creates the data directory if it doesn't exist
func (c *Config) EnsureDataDir() error {
	return os.MkdirAll(c.DataDir, 0755)
}

// DatabasePath returns the path to the SQLite database
func (c *Config) DatabasePath() string {
	return filepath.Join(c.DataDir, "tcc-bridge.db")
}
