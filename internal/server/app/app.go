package app

import (
	"embed"
	"io/fs"
	"log"

	"github.com/gotunnel/internal/server/config"
	httpapi "github.com/gotunnel/internal/server/http"
	"github.com/gotunnel/internal/server/service"
	db "github.com/gotunnel/internal/server/storage/sqlite"
	"github.com/gotunnel/pkg/auth"
)

//go:embed all:dist/*
var staticFiles embed.FS

type webStore interface {
	db.ClientStore
	db.InstallTokenStore
	db.TrafficStore
	db.OperationalEventStore
}

// WebServer Web控制台服务
type WebServer struct {
	Store     webStore
	Server    httpapi.ServerInterface
	ConfigSvc service.ConfigService
}

// NewWebServer 创建Web服务
func NewWebServer(store webStore, srv httpapi.ServerInterface, cfg *config.ServerConfig, cfgPath string) *WebServer {
	return &WebServer{
		Store:     store,
		Server:    srv,
		ConfigSvc: service.NewConfigService(cfg, cfgPath, srv),
	}
}

// Run starts the embedded web console with JWT authentication enabled.
func (w *WebServer) Run(addr, username, password, jwtSecret string) error {
	r := httpapi.New()

	jwtAuth := auth.NewJWTAuth(jwtSecret, 24)

	r.SetupRoutes(httpapi.Dependencies{
		ClientStore:       w.Store,
		InstallTokenStore: w.Store,
		ServerRuntime:     w.Server,
		ConfigService:     w.ConfigSvc,
		TrafficStore:      w.Store,
		OperationalEvents: w.Store,
		JWTAuth:           jwtAuth,
		Username:          username,
		Password:          password,
	})

	staticFS, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		return err
	}
	r.SetupStaticFiles(staticFS)

	log.Printf("[Web] Console listening on %s", addr)
	return r.Engine.Run(addr)
}
