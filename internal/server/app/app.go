package app

import (
	"embed"
	"io/fs"
	"log"

	"github.com/gotunnel/internal/server/config"
	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/router"
	"github.com/gotunnel/pkg/auth"
)

//go:embed all:dist/*
var staticFiles embed.FS

// WebServer Web控制台服务
type WebServer struct {
	ClientStore  db.ClientStore
	Server       router.ServerInterface
	Config       *config.ServerConfig
	ConfigPath   string
	TrafficStore db.TrafficStore
}

// NewWebServer 创建Web服务
func NewWebServer(cs db.ClientStore, srv router.ServerInterface, cfg *config.ServerConfig, cfgPath string, store db.Store) *WebServer {
	return &WebServer{
		ClientStore:  cs,
		Server:       srv,
		Config:       cfg,
		ConfigPath:   cfgPath,
		TrafficStore: store,
	}
}

// Run 启动Web服务 (无认证，仅用于开发)
func (w *WebServer) Run(addr string) error {
	r := router.New()

	// 使用默认凭据和 JWT
	jwtAuth := auth.NewJWTAuth("dev-secret", 24)
	r.SetupRoutes(w, jwtAuth, "admin", "admin")

	// 静态文件
	staticFS, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		return err
	}
	r.SetupStaticFiles(staticFS)

	log.Printf("[Web] Console listening on %s", addr)
	return r.Engine.Run(addr)
}

// RunWithAuth 启动带 Basic Auth 的 Web 服务 (已废弃，使用 RunWithJWT)
func (w *WebServer) RunWithAuth(addr, username, password string) error {
	// 转发到 JWT 认证
	return w.RunWithJWT(addr, username, password, "auto-generated-secret")
}

// RunWithJWT 启动带 JWT 认证的 Web 服务
func (w *WebServer) RunWithJWT(addr, username, password, jwtSecret string) error {
	r := router.New()

	// JWT 认证器
	jwtAuth := auth.NewJWTAuth(jwtSecret, 24) // 24小时过期

	// 设置所有路由
	r.SetupRoutes(w, jwtAuth, username, password)

	// 静态文件
	staticFS, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		return err
	}
	r.SetupStaticFiles(staticFS)

	log.Printf("[Web] Console listening on %s (JWT auth enabled)", addr)
	return r.Engine.Run(addr)
}

// GetClientStore 获取客户端存储
func (w *WebServer) GetClientStore() db.ClientStore {
	return w.ClientStore
}

// GetServer 获取服务端接口
func (w *WebServer) GetServer() router.ServerInterface {
	return w.Server
}

// GetConfig 获取配置
func (w *WebServer) GetConfig() *config.ServerConfig {
	return w.Config
}

// GetConfigPath 获取配置文件路径
func (w *WebServer) GetConfigPath() string {
	return w.ConfigPath
}

// SaveConfig 保存配置
func (w *WebServer) SaveConfig() error {
	return config.SaveServerConfig(w.ConfigPath, w.Config)
}

// GetTrafficStore 获取流量存储
func (w *WebServer) GetTrafficStore() db.TrafficStore {
	return w.TrafficStore
}
