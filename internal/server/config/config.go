package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// ServerConfig 服务端配置
type ServerConfig struct {
	Server ServerSettings `yaml:"server"`
	Web    WebSettings    `yaml:"web"`
}

// ServerSettings 服务端设置
type ServerSettings struct {
	BindAddr         string `yaml:"bind_addr"`
	BindPort         int    `yaml:"bind_port"`
	Token            string `yaml:"token"`
	HeartbeatSec     int    `yaml:"heartbeat_sec"`
	HeartbeatTimeout int    `yaml:"heartbeat_timeout"`
	DBPath           string `yaml:"db_path"`
}

// WebSettings Web控制台设置
type WebSettings struct {
	Enabled  bool   `yaml:"enabled"`
	BindAddr string `yaml:"bind_addr"`
	BindPort int    `yaml:"bind_port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// LoadServerConfig 加载服务端配置
func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg ServerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 设置默认值
	if cfg.Server.BindAddr == "" {
		cfg.Server.BindAddr = "0.0.0.0"
	}
	if cfg.Server.BindPort == 0 {
		cfg.Server.BindPort = 7000
	}
	if cfg.Server.HeartbeatSec == 0 {
		cfg.Server.HeartbeatSec = 30
	}
	if cfg.Server.HeartbeatTimeout == 0 {
		cfg.Server.HeartbeatTimeout = 90
	}
	if cfg.Server.DBPath == "" {
		cfg.Server.DBPath = "gotunnel.db"
	}

	// Web 默认值
	if cfg.Web.BindAddr == "" {
		cfg.Web.BindAddr = "0.0.0.0"
	}
	if cfg.Web.BindPort == 0 {
		cfg.Web.BindPort = 7500
	}

	return &cfg, nil
}
