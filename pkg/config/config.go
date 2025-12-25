package config

import (
	"os"

	"github.com/gotunnel/pkg/protocol"
	"gopkg.in/yaml.v3"
)

// ServerConfig 服务端配置
type ServerConfig struct {
	Server  ServerSettings `yaml:"server"`
	Clients []ClientConfig `yaml:"clients"`
}

// ServerSettings 服务端设置
type ServerSettings struct {
	BindAddr      string `yaml:"bind_addr"`
	BindPort      int    `yaml:"bind_port"`
	Token         string `yaml:"token"`
	HeartbeatSec  int    `yaml:"heartbeat_sec"`
	HeartbeatTimeout int `yaml:"heartbeat_timeout"`
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
