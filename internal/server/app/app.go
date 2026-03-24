package app

import (
	"crypto/tls"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"

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
	TLSConfig *tls.Config
}

// NewWebServer 创建Web服务
func NewWebServer(store webStore, srv httpapi.ServerInterface, cfg *config.ServerConfig, cfgPath string, tlsConfig *tls.Config) *WebServer {
	return &WebServer{
		Store:     store,
		Server:    srv,
		ConfigSvc: service.NewConfigService(cfg, cfgPath, srv),
		TLSConfig: tlsConfig,
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
	if w.TLSConfig == nil {
		return fmt.Errorf("web TLS config is required")
	}

	server := &http.Server{
		Addr:      addr,
		Handler:   r.Engine,
		TLSConfig: w.TLSConfig.Clone(),
	}
	listener, err := tls.Listen("tcp", addr, server.TLSConfig)
	if err != nil {
		return err
	}
	return server.Serve(listener)
}
