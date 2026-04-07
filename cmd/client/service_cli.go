package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	clientconfig "github.com/gotunnel/internal/client/config"
)

func runServiceCLI(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gotunnel-client service <install|start|stop|restart|status|uninstall> [options]")
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "install":
		return runServiceInstallCLI(args[1:])
	case "start", "stop", "restart", "status", "uninstall":
		return runServiceControlCLI(strings.ToLower(strings.TrimSpace(args[0])), args[1:])
	default:
		return fmt.Errorf("unsupported service subcommand %q", args[0])
	}
}

func runServiceInstallCLI(args []string) error {
	fs := flag.NewFlagSet("service install", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	server := fs.String("s", "", "server address (ip:port)")
	token := fs.String("t", "", "auth token")
	configPath := fs.String("c", "", "config file path")
	dataDir := fs.String("data-dir", "", "client data directory")
	clientName := fs.String("name", "", "client display name")
	clientID := fs.String("id", "", "client id")
	reconnectMin := fs.Int("reconnect-min", 0, "minimum reconnect delay in seconds")
	reconnectMax := fs.Int("reconnect-max", 0, "maximum reconnect delay in seconds")
	noTLS := fs.Bool("no-tls", false, "disable TLS")
	serviceName := fs.String("service-name", defaultServiceName(), "service name / label")
	serviceDisplayName := fs.String("service-display-name", defaultServiceDisplayName(), "service display name")
	serviceLogPath := fs.String("service-log-file", "", "service log file path")

	if err := fs.Parse(args); err != nil {
		return err
	}

	resolvedDataDir := strings.TrimSpace(*dataDir)
	if resolvedDataDir == "" {
		resolvedDataDir = defaultManagedDataDir()
	}
	resolvedConfigPath := strings.TrimSpace(*configPath)
	if resolvedConfigPath == "" {
		resolvedConfigPath = defaultManagedConfigPath(resolvedDataDir)
	}

	cfg := &clientconfig.ClientConfig{}
	if existing, err := maybeLoadClientConfig(resolvedConfigPath); err != nil {
		return err
	} else if existing != nil {
		cfg = existing
	}

	if *server != "" {
		cfg.Server = *server
	}
	if *token != "" {
		cfg.Token = *token
	}
	cfg.NoTLS = *noTLS
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
	cfg.DataDir = resolvedDataDir

	if err := promptForMissingConnectionValues(&cfg.Server, &cfg.Token); err != nil {
		return fmt.Errorf("read connection settings: %w", err)
	}

	if strings.TrimSpace(cfg.Server) == "" || strings.TrimSpace(cfg.Token) == "" {
		return fmt.Errorf("service install requires -s and -t, or an existing config file with server and token; interactive prompt is available in a terminal")
	}

	resolvedConfigPath, err := filepath.Abs(resolvedConfigPath)
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	if err := clientconfig.SaveClientConfig(resolvedConfigPath, cfg); err != nil {
		return err
	}

	if handled, err := maybeHandleServiceCommand(serviceCommandOptions{
		Action:      "install",
		Name:        *serviceName,
		DisplayName: *serviceDisplayName,
		ConfigPath:  resolvedConfigPath,
		LogPath:     *serviceLogPath,
	}, cfg); err != nil {
		return err
	} else if !handled {
		return fmt.Errorf("service install handler was not executed")
	}

	fmt.Printf("Config written to %s\n", resolvedConfigPath)
	return nil
}

func runServiceControlCLI(action string, args []string) error {
	fs := flag.NewFlagSet("service "+action, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	serviceName := fs.String("service-name", defaultServiceName(), "service name / label")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if handled, err := maybeHandleServiceCommand(serviceCommandOptions{
		Action: action,
		Name:   *serviceName,
	}, nil); err != nil {
		return err
	} else if !handled {
		return fmt.Errorf("service %s handler was not executed", action)
	}
	return nil
}

func maybeLoadClientConfig(path string) (*clientconfig.ClientConfig, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat config file: %w", err)
	}
	cfg, err := clientconfig.LoadClientConfig(path)
	if err != nil {
		return nil, fmt.Errorf("load config file: %w", err)
	}
	return cfg, nil
}
