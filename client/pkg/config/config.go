package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ServerAddr   string `json:"server_addr"`
	UseTLS       bool   `json:"use_tls"`
	TempDir      string `json:"temp_dir"`
	LastUser     string `json:"last_user"`
	LastDeviceID string `json:"last_device_id"`
}

var (
	DefaultConfigPath = "mandala-workspace/config.json"
)

func GetConfigDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(configDir, "mandala-workspace")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return "", err
		}
	}
	return dir, nil
}

func Load() (*Config, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "config.json")
	
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Return default config if it doesn't exist
		return &Config{
			ServerAddr: "localhost:50051",
			UseTLS:     false, // Default to false for development
			TempDir:    filepath.Join(os.TempDir(), "mandala-workspace"),
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Save() error {
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "config.json")

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (c *Config) String() string {
	return fmt.Sprintf("ServerAddr: %s, TLS: %v, TempDir: %s", c.ServerAddr, c.UseTLS, c.TempDir)
}
