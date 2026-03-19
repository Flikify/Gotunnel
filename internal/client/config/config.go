package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// ClientConfig 客户端配置
type ClientConfig struct {
	Server string `yaml:"server"` // 服务器地址
	Token  string `yaml:"token"`  // 认证 Token
	NoTLS  bool   `yaml:"no_tls"` // 禁用 TLS
}

// LoadClientConfig 加载客户端配置
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
