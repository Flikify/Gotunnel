package main

import (
	"context"
	"flag"
	"log"
	"os"
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
	if len(os.Args) > 1 && os.Args[1] == "service" {
		if err := runServiceCLI(os.Args[2:]); err != nil {
			log.Fatalf("Service command failed: %v", err)
		}
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "desktop-helper" || os.Args[1] == "desktop-agent") {
		if err := runDesktopHelperCLI(os.Args[2:]); err != nil {
			log.Fatalf("Desktop agent failed: %v", err)
		}
		return
	}

	server := flag.String("s", "", "server address (ip:port)")
	token := flag.String("t", "", "auth token")
	configPath := flag.String("c", "", "config file path")
	dataDir := flag.String("data-dir", "", "client data directory")
	clientName := flag.String("name", "", "client display name")
	clientID := flag.String("id", "", "client id")
	reconnectMin := flag.Int("reconnect-min", 0, "minimum reconnect delay in seconds")
	reconnectMax := flag.Int("reconnect-max", 0, "maximum reconnect delay in seconds")
	serviceMode := flag.Bool("service", false, "run as a managed Windows service")
	serviceAction := flag.String("service-action", "", "manage background service: install|uninstall|start|stop|restart|status")
	serviceName := flag.String("service-name", defaultServiceName(), "service name / label")
	serviceDisplayName := flag.String("service-display-name", defaultServiceDisplayName(), "service display name")
	serviceLogPath := flag.String("log", "", "service log file path")
	flag.String("service-log-file", "", "deprecated: use -log instead")
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

	if err := promptForMissingConnectionValues(&cfg.Server, &cfg.Token); err != nil {
		log.Fatalf("Failed to read connection settings: %v", err)
	}

	if handled, err := maybeHandleServiceCommand(serviceCommandOptions{
		Action:      *serviceAction,
		Name:        *serviceName,
		DisplayName: *serviceDisplayName,
		ConfigPath:  *configPath,
		LogPath:     *serviceLogPath,
	}, cfg); handled {
		if err != nil {
			log.Fatalf("Service command failed: %v", err)
		}
		return
	}

	if cfg.Server == "" || cfg.Token == "" {
		log.Fatal("Usage: client [-c config.yaml] | [-s <server:port> -t <token>] (interactive prompt is available in a terminal)")
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
