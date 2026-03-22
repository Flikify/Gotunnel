package main

import (
	"flag"
	"log"
	"time"

	"github.com/gotunnel/internal/client/config"
	"github.com/gotunnel/internal/client/tunnel"
	"github.com/gotunnel/pkg/crypto"
	"github.com/gotunnel/pkg/version"
)

// Version information injected by ldflags.
var Version string
var BuildTime string
var GitCommit string

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

	opts := tunnel.ClientOptions{
		DataDir:    cfg.DataDir,
		ClientID:   cfg.ClientID,
		ClientName: cfg.Name,
	}
	if cfg.ReconnectMinSec > 0 {
		opts.ReconnectDelay = time.Duration(cfg.ReconnectMinSec) * time.Second
	}
	if cfg.ReconnectMaxSec > 0 {
		opts.ReconnectMaxDelay = time.Duration(cfg.ReconnectMaxSec) * time.Second
	}

	client := tunnel.NewClientWithOptions(cfg.Server, cfg.Token, opts)

	if !cfg.NoTLS {
		client.TLSEnabled = true
		client.TLSConfig = crypto.ClientTLSConfig()
		log.Printf("[Client] TLS enabled")
	}

	if err := client.Run(); err != nil {
		log.Fatalf("Client stopped: %v", err)
	}
}
