package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"

	"gopkg.in/yaml.v3"
)

// ServerConfig 服务端配置
type ServerConfig struct {
	Server      ServerSettings      `yaml:"server"`
	Web         WebSettings         `yaml:"web"`
	PluginStore PluginStoreSettings `yaml:"plugin_store"`
	JSPlugins   []JSPluginConfig    `yaml:"js_plugins,omitempty"`
}

// JSPluginConfig JS 插件配置
type JSPluginConfig struct {
	Name      string            `yaml:"name"`
	Path      string            `yaml:"path"`                 // JS 文件路径
	SigPath   string            `yaml:"sig_path,omitempty"`   // 签名文件路径 (默认为 path + ".sig")
	AutoPush  []string          `yaml:"auto_push,omitempty"`  // 自动推送到的客户端 ID 列表
	Config    map[string]string `yaml:"config,omitempty"`     // 插件配置
	AutoStart bool              `yaml:"auto_start,omitempty"` // 是否自动启动
}

// PluginStoreSettings 插件仓库设置
type PluginStoreSettings struct {
	URL string `yaml:"url"` // 插件仓库 URL，为空则使用默认值
}

// 默认插件仓库 URL
const DefaultPluginStoreURL = "https://git.92coco.cn/flik/GoTunnel-Plugins/raw/branch/main/store.json"

// GetPluginStoreURL 获取插件仓库 URL
func (s *PluginStoreSettings) GetPluginStoreURL() string {
	if s.URL != "" {
		return s.URL
	}
	return DefaultPluginStoreURL
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
	n, err := rand.Read(bytes)
	if err != nil || n != len(bytes) {
		// 安全关键：随机数生成失败时 panic
		panic("crypto/rand failed: unable to generate secure token")
	}
	return hex.EncodeToString(bytes)
}

// GenerateWebCredentials 生成 Web 控制台凭据
func GenerateWebCredentials(cfg *ServerConfig) bool {
	if cfg.Web.Username == "" {
		cfg.Web.Username = "admin"
	}
	if cfg.Web.Password == "" {
		cfg.Web.Password = generateToken(16)
		return true // 表示生成了新密码
	}
	return false
}

// SaveServerConfig 保存服务端配置
func SaveServerConfig(path string, cfg *ServerConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
