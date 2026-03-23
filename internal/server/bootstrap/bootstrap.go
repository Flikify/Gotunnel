package bootstrap

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gotunnel/internal/server/app"
	"github.com/gotunnel/internal/server/config"
	runtime "github.com/gotunnel/internal/server/runtime"
	db "github.com/gotunnel/internal/server/storage/sqlite"
	"github.com/gotunnel/pkg/crypto"
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

	log.Printf("[Server] Token: %s", cfg.Server.Token)

	server := runtime.NewServer(
		store,
		cfg.Server.BindAddr,
		cfg.Server.BindPort,
		cfg.Server.Token,
		cfg.Server.HeartbeatSec,
		cfg.Server.HeartbeatTimeout,
	)
	server.ApplyRuntimeConfig(
		cfg.Server.HeartbeatSec,
		cfg.Server.HeartbeatTimeout,
		cfg.Server.MaxClientProxies,
		cfg.Server.ClientResponseTimeoutSec,
	)
	server.SetTrafficStore(store)

	if !cfg.Server.TLSDisabled {
		tlsConfig, err := crypto.GenerateTLSConfig()
		if err != nil {
			return fmt.Errorf("generate TLS config: %w", err)
		}
		server.SetTLSConfig(tlsConfig)
		log.Printf("[Server] TLS enabled")
	}

	if cfg.Server.Web.Enabled {
		if config.GenerateWebCredentials(cfg) {
			log.Printf("[Web] Auto-generated credentials - Username: %s, Password: %s",
				cfg.Server.Web.Username, cfg.Server.Web.Password)
			log.Printf("[Web] Please save these credentials and update your config file")
			if err := config.SaveServerConfig(configPath, cfg); err != nil {
				log.Printf("[Web] Warning: failed to save config: %v", err)
			}
		}

		webServer := app.NewWebServer(store, server, cfg, configPath)
		addr := fmt.Sprintf("%s:%d", cfg.Server.BindAddr, cfg.Server.Web.BindPort)
		go func() {
			if err := webServer.Run(addr, cfg.Server.Web.Username, cfg.Server.Web.Password, cfg.Server.Token); err != nil {
				log.Printf("[Web] Server error: %v", err)
			}
		}()
		log.Printf("[Web] Console running at http://%s (authentication required)", addr)
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
