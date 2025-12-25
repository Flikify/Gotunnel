package config

import (
	"crypto/rand"
	"encoding/hex"
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
	TLSDisabled      bool   `yaml:"tls_disabled"` // 默认启用 TLS，设置为 true 禁用
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
	var cfg ServerConfig

	// 尝试读取配置文件，不存在则使用默认配置
	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	}

	// 设置默认值
	setDefaults(&cfg)

	return &cfg, nil
}

// setDefaults 设置默认值
func setDefaults(cfg *ServerConfig) {
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

	// Web 默认启用
	if cfg.Web.BindAddr == "" {
		cfg.Web.BindAddr = "0.0.0.0"
	}
	if cfg.Web.BindPort == 0 {
		cfg.Web.BindPort = 7500
		cfg.Web.Enabled = true
	}

	// Token 未配置时自动生成 32 位
	if cfg.Server.Token == "" {
		cfg.Server.Token = generateToken(32)
	}
}

// generateToken 生成随机 token
func generateToken(length int) string {
	bytes := make([]byte, length/2)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// SaveServerConfig 保存服务端配置
func SaveServerConfig(path string, cfg *ServerConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
