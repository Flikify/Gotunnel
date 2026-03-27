package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ClientConfig defines client runtime configuration.
type ClientConfig struct {
	Server          string `yaml:"server"`
	Token           string `yaml:"token"`
	NoTLS           bool   `yaml:"no_tls"`
	DataDir         string `yaml:"data_dir"`
	Name            string `yaml:"name"`
	ClientID        string `yaml:"client_id"`
	ReconnectMinSec int    `yaml:"reconnect_min_sec"`
	ReconnectMaxSec int    `yaml:"reconnect_max_sec"`
}

// LoadClientConfig loads client configuration from YAML.
func LoadClientConfig(path string) (*ClientConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg ClientConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// SaveClientConfig persists client configuration as YAML.
func SaveClientConfig(path string, cfg *ClientConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
