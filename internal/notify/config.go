package notify

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// DefaultConfigPath returns the default path for the notification config file.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "nameport", "notify.json")
}

// LoadConfig reads notification config from path. If the file does not exist,
// it returns DefaultConfig.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	// Ensure EventFilter is initialized
	if cfg.EventFilter == nil {
		cfg.EventFilter = make(map[EventType]bool)
	}

	return cfg, nil
}

// SaveConfig writes notification config to path as JSON.
func SaveConfig(path string, cfg Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0666)
}
