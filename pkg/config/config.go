package config

import (
	"os"

	"github.com/gotunnel/pkg/protocol"
	"gopkg.in/yaml.v3"
)

// ServerConfig 服务端配置
type ServerConfig struct {
	Server  ServerSettings `yaml:"server"`
	Web     WebSettings    `yaml:"web"`
	Clients []ClientConfig `yaml:"clients"`
}

// ServerSettings 服务端设置
type ServerSettings struct {
	BindAddr         string `yaml:"bind_addr"`
	BindPort         int    `yaml:"bind_port"`
	Token            string `yaml:"token"`
	HeartbeatSec     int    `yaml:"heartbeat_sec"`
	HeartbeatTimeout int    `yaml:"heartbeat_timeout"`
}

// WebSettings Web控制台设置
type WebSettings struct {
	Enabled  bool   `yaml:"enabled"`
	BindAddr string `yaml:"bind_addr"`
	BindPort int    `yaml:"bind_port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// ClientConfig 客户端配置（服务端维护）
type ClientConfig struct {
	ID    string              `yaml:"id"`
	Rules []protocol.ProxyRule `yaml:"rules"`
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
	if cfg.Server.HeartbeatSec == 0 {
		cfg.Server.HeartbeatSec = 30
	}
	if cfg.Server.HeartbeatTimeout == 0 {
		cfg.Server.HeartbeatTimeout = 90
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

// GetClientRules 获取指定客户端的代理规则
func (c *ServerConfig) GetClientRules(clientID string) []protocol.ProxyRule {
	for _, client := range c.Clients {
		if client.ID == clientID {
			return client.Rules
		}
	}
	return nil
}

// SaveServerConfig 保存服务端配置
func SaveServerConfig(path string, cfg *ServerConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
