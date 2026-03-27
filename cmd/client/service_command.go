package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	clientconfig "github.com/gotunnel/internal/client/config"
)

type serviceCommandOptions struct {
	Action      string
	Name        string
	DisplayName string
	ConfigPath  string
	LogPath     string
}

func defaultServiceName() string {
	switch runtime.GOOS {
	case "windows":
		return "GoTunnelClient"
	case "darwin":
		return "com.gotunnel.client"
	default:
		return "gotunnel-client"
	}
}

func defaultServiceDisplayName() string {
	return "GoTunnel Client"
}

func defaultManagedDataDir() string {
	switch runtime.GOOS {
	case "windows":
		if root := strings.TrimSpace(os.Getenv("ProgramData")); root != "" {
			return filepath.Join(root, "GoTunnel")
		}
		return filepath.Join(`C:\ProgramData`, "GoTunnel")
	case "darwin":
		return filepath.Join("/Library/Application Support", "GoTunnel")
	default:
		return "/var/lib/gotunnel"
	}
}

func defaultManagedConfigPath(dataDir string) string {
	if strings.TrimSpace(dataDir) == "" {
		dataDir = defaultManagedDataDir()
	}
	return filepath.Join(dataDir, "client.yaml")
}

func maybeHandleServiceCommand(opts serviceCommandOptions, cfg *clientconfig.ClientConfig) (bool, error) {
	if strings.TrimSpace(opts.Action) == "" {
		return false, nil
	}

	normalized, err := normalizeServiceCommandOptions(opts, cfg)
	if err != nil {
		return true, err
	}
	return true, runServiceCommand(normalized, cfg)
}

func normalizeServiceCommandOptions(opts serviceCommandOptions, cfg *clientconfig.ClientConfig) (serviceCommandOptions, error) {
	opts.Action = strings.ToLower(strings.TrimSpace(opts.Action))
	switch opts.Action {
	case "install", "uninstall", "start", "stop", "restart", "status":
	default:
		return opts, fmt.Errorf("unsupported -service-action %q", opts.Action)
	}

	opts.Name = strings.TrimSpace(opts.Name)
	if opts.Name == "" {
		opts.Name = defaultServiceName()
	}

	opts.DisplayName = strings.TrimSpace(opts.DisplayName)
	if opts.DisplayName == "" {
		opts.DisplayName = defaultServiceDisplayName()
	}

	if opts.ConfigPath != "" {
		abs, err := filepath.Abs(opts.ConfigPath)
		if err != nil {
			return opts, fmt.Errorf("resolve config path: %w", err)
		}
		opts.ConfigPath = abs
	}

	if opts.Action == "install" && opts.ConfigPath == "" {
		return opts, fmt.Errorf("-c is required for -service-action install")
	}

	if opts.LogPath != "" {
		abs, err := filepath.Abs(opts.LogPath)
		if err != nil {
			return opts, fmt.Errorf("resolve service log path: %w", err)
		}
		opts.LogPath = abs
	}

	if opts.LogPath == "" {
		dataDir := resolveServiceDataDir(cfg, opts.ConfigPath)
		if dataDir != "" {
			opts.LogPath = filepath.Join(dataDir, "service.log")
		}
	}

	return opts, nil
}

func resolveServiceDataDir(cfg *clientconfig.ClientConfig, configPath string) string {
	if cfg != nil {
		if dataDir := strings.TrimSpace(cfg.DataDir); dataDir != "" {
			if abs, err := filepath.Abs(dataDir); err == nil {
				return abs
			}
			return dataDir
		}
	}

	if configPath != "" {
		return filepath.Dir(configPath)
	}
	return ""
}

func currentExecutablePath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil && strings.TrimSpace(resolved) != "" {
		path = resolved
	}
	return filepath.Abs(path)
}
