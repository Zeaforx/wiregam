package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DefaultProvider string `json:"default_provider"`
}

// StoreDefaultProvider records the provider used when a command omits one.
func StoreDefaultProvider(provider string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".wiregam")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := json.MarshalIndent(Config{DefaultProvider: provider}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	if err := os.WriteFile(filepath.Join(configDir, "config.json"), data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
