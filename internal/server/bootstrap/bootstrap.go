package bootstrap

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gotunnel/internal/server/app"
	"github.com/gotunnel/internal/server/config"
	runtime "github.com/gotunnel/internal/server/runtime"
	db "github.com/gotunnel/internal/server/storage/sqlite"
	"github.com/gotunnel/pkg/crypto"
	"github.com/gotunnel/pkg/observability"
)

// Run assembles and starts the server process from a config path.
func Run(configPath string) error {
	cfg, err := config.LoadServerConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	store, err := db.NewSQLiteStore(cfg.Server.DBPath)
	if err != nil {
		return fmt.Errorf("init database: %w", err)
	}
	defer store.Close()

	server := runtime.NewServer(
		store,
		cfg.Server.BindAddr,
		cfg.Server.BindPort,
		cfg.Server.Token,
		cfg.Server.HeartbeatSec,
		cfg.Server.HeartbeatTimeout,
	)
	server.SetOperationalEventStore(store)
	server.ApplyRuntimeConfig(
		cfg.Server.HeartbeatSec,
		cfg.Server.HeartbeatTimeout,
		cfg.Server.MaxClientProxies,
		cfg.Server.ClientResponseTimeoutSec,
	)
	server.SetTrafficStore(store)

	diagRoot := filepath.Join(filepath.Dir(cfg.Server.DBPath), "server-diagnostics")
	diagStore, err := observability.NewDiagnosticStore(observability.StoreOptions{
		RootDir:       diagRoot,
		RetentionDays: 14,
		NodeID:        "server",
		NodeRole:      observability.NodeRoleServer,
	})
	if err != nil {
		return fmt.Errorf("init diagnostic store: %w", err)
	}
	server.SetDiagnosticStore(diagStore)
	log.SetFlags(0)
	log.SetOutput(io.MultiWriter(os.Stderr, observability.NewStdLogWriter(diagStore, "server", map[string]string{
		"channel": "server-legacy",
	})))

	var tunnelTLSConfig *tls.Config
	if !cfg.Server.TLSDisabled {
		tunnelTLSConfig, err = crypto.GenerateTLSConfig()
		if err != nil {
			return fmt.Errorf("generate TLS config: %w", err)
		}
		server.SetTLSConfig(tunnelTLSConfig)
		log.Printf("[Server] TLS enabled")
	}

	if cfg.Server.Web.Enabled {
		generatedWebCreds := config.GenerateWebCredentials(cfg)
		generatedWebJWT := config.GenerateWebJWTSecret(cfg)
		if generatedWebCreds {
			log.Printf("[Web] Auto-generated credentials - Username: %s, Password: %s",
				cfg.Server.Web.Username, cfg.Server.Web.Password)
			log.Printf("[Web] Please save these credentials and update your config file")
		}
		if generatedWebJWT {
			log.Printf("[Web] Generated isolated JWT signing secret for the web console")
		}
		if generatedWebCreds || generatedWebJWT {
			if err := config.SaveServerConfig(configPath, cfg); err != nil {
				log.Printf("[Web] Warning: failed to save config: %v", err)
			}
		}

		webTLSConfig := tunnelTLSConfig
		if webTLSConfig == nil {
			webTLSConfig, err = crypto.GenerateTLSConfig()
			if err != nil {
				return fmt.Errorf("generate web TLS config: %w", err)
			}
		}

		webServer := app.NewWebServer(store, server, cfg, configPath, webTLSConfig)
		addr := fmt.Sprintf("%s:%d", cfg.Server.BindAddr, cfg.Server.Web.BindPort)
		go func() {
			if err := webServer.Run(addr, cfg.Server.Web.Username, cfg.Server.Web.Password, cfg.Server.Web.JWTSecret); err != nil {
				log.Printf("[Web] Server error: %v", err)
			}
		}()
		log.Printf("[Web] Console running at https://%s (authentication required)", addr)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Printf("[Server] Received shutdown signal")
		server.Shutdown(30 * time.Second)
		os.Exit(0)
	}()

	return server.Run()
}
