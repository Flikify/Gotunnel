package app

import (
	"embed"
	"io/fs"
	"log"

	"github.com/gotunnel/internal/server/config"
	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/router"
	"github.com/gotunnel/internal/server/service"
	"github.com/gotunnel/pkg/auth"
)

//go:embed all:dist/*
var staticFiles embed.FS

type webStore interface {
	db.ClientStore
	db.InstallTokenStore
	db.TrafficStore
}

// WebServer Web控制台服务
type WebServer struct {
	Store     webStore
	Server    router.ServerInterface
	ConfigSvc service.ConfigService
}

// NewWebServer 创建Web服务
func NewWebServer(store webStore, srv router.ServerInterface, cfg *config.ServerConfig, cfgPath string) *WebServer {
	return &WebServer{
		Store:     store,
		Server:    srv,
		ConfigSvc: service.NewConfigService(cfg, cfgPath, srv),
	}
}

// Run 启动Web服务 (无认证，仅用于开发)
func (w *WebServer) Run(addr string) error {
	r := router.New()

	// 使用默认凭据和 JWT
	jwtAuth := auth.NewJWTAuth("dev-secret", 24)
	r.SetupRoutes(router.Dependencies{
		ClientStore:       w.Store,
		InstallTokenStore: w.Store,
		ServerRuntime:     w.Server,
		ConfigService:     w.ConfigSvc,
		TrafficStore:      w.Store,
		JWTAuth:           jwtAuth,
		Username:          "admin",
		Password:          "admin",
	})

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
	r.SetupRoutes(router.Dependencies{
		ClientStore:       w.Store,
		InstallTokenStore: w.Store,
		ServerRuntime:     w.Server,
		ConfigService:     w.ConfigSvc,
		TrafficStore:      w.Store,
		JWTAuth:           jwtAuth,
		Username:          username,
		Password:          password,
	})

	// 静态文件
	staticFS, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		return err
	}
	r.SetupStaticFiles(staticFS)

	log.Printf("[Web] Console listening on %s (JWT auth enabled)", addr)
	return r.Engine.Run(addr)
}
