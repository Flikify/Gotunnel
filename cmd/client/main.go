package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
	"time"

	clientapp "github.com/gotunnel/internal/client/app"
	"github.com/gotunnel/internal/client/config"
	"github.com/gotunnel/pkg/version"
)

// Version information injected by ldflags.
var Version string
var BuildTime string
var GitCommit string

type runtimeOptions struct {
	AppConfig      clientapp.Config
	ServiceMode    bool
	ServiceName    string
	ServiceLogPath string
}

func init() {
	version.SetVersion(Version)
	version.SetBuildInfo(GitCommit, BuildTime)
}

func main() {
	server := flag.String("s", "", "server address (ip:port)")
	token := flag.String("t", "", "auth token")
	configPath := flag.String("c", "", "config file path")
	dataDir := flag.String("data-dir", "", "client data directory")
	clientName := flag.String("name", "", "client display name")
	clientID := flag.String("id", "", "client id")
	reconnectMin := flag.Int("reconnect-min", 0, "minimum reconnect delay in seconds")
	reconnectMax := flag.Int("reconnect-max", 0, "maximum reconnect delay in seconds")
	serviceMode := flag.Bool("service", false, "run as a managed Windows service")
	serviceName := flag.String("service-name", "GoTunnelClient", "Windows service name")
	serviceLogPath := flag.String("service-log-file", "", "path to the Windows service bootstrap log")
	flag.Parse()

	var cfg *config.ClientConfig
	if *configPath != "" {
		var err error
		cfg, err = config.LoadClientConfig(*configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	} else {
		cfg = &config.ClientConfig{}
	}

	if *server != "" {
		cfg.Server = *server
	}
	if *token != "" {
		cfg.Token = *token
	}
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}
	if *clientName != "" {
		cfg.Name = *clientName
	}
	if *clientID != "" {
		cfg.ClientID = *clientID
	}
	if *reconnectMin > 0 {
		cfg.ReconnectMinSec = *reconnectMin
	}
	if *reconnectMax > 0 {
		cfg.ReconnectMaxSec = *reconnectMax
	}

	if cfg.Server == "" || cfg.Token == "" {
		log.Fatal("Usage: client [-c config.yaml] | [-s <server:port> -t <token>]")
	}

	opts := runtimeOptions{
		AppConfig: clientapp.Config{
			Server:     cfg.Server,
			Token:      cfg.Token,
			DataDir:    cfg.DataDir,
			ClientID:   cfg.ClientID,
			ClientName: cfg.Name,
			TLSEnabled: !cfg.NoTLS,
			ReconnectDelay: func() time.Duration {
				if cfg.ReconnectMinSec <= 0 {
					return 0
				}
				return time.Duration(cfg.ReconnectMinSec) * time.Second
			}(),
			ReconnectMaxDelay: func() time.Duration {
				if cfg.ReconnectMaxSec <= 0 {
					return 0
				}
				return time.Duration(cfg.ReconnectMaxSec) * time.Second
			}(),
		},
		ServiceMode:    *serviceMode,
		ServiceName:    *serviceName,
		ServiceLogPath: *serviceLogPath,
	}

	if err := runClient(opts); err != nil {
		log.Fatalf("Client stopped: %v", err)
	}
}

func runConsoleClient(cfg clientapp.Config) error {
	app := clientapp.NewService()
	app.Configure(cfg)

	if cfg.TLSEnabled {
		log.Printf("[Client] TLS enabled")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.RunContext(ctx); err != nil {
		return err
	}

	<-ctx.Done()
	log.Printf("[Client] Shutting down")
	return nil
}
