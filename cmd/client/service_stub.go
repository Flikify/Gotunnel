//go:build !windows

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	clientconfig "github.com/gotunnel/internal/client/config"
)

func runClient(opts runtimeOptions) error {
	if opts.ServiceMode {
		return fmt.Errorf("-service is only supported on Windows")
	}
	return runConsoleClient(opts.AppConfig)
}

func runServiceCommand(opts serviceCommandOptions, cfg *clientconfig.ClientConfig) error {
	switch runtime.GOOS {
	case "linux":
		return runLinuxServiceCommand(opts, cfg)
	case "darwin":
		return runDarwinServiceCommand(opts, cfg)
	default:
		return fmt.Errorf("-service-action is not supported on %s", runtime.GOOS)
	}
}

func runLinuxServiceCommand(opts serviceCommandOptions, cfg *clientconfig.ClientConfig) error {
	unitName := linuxUnitName(opts.Name)
	unitPath := filepath.Join("/etc/systemd/system", unitName)

	switch opts.Action {
	case "install":
		exePath, err := currentExecutablePath()
		if err != nil {
			return err
		}
		workingDir := resolveWorkingDir(cfg, opts.ConfigPath)
		unit := fmt.Sprintf(`[Unit]
Description=%s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart="%s" -c "%s"
WorkingDirectory=%s
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`, opts.DisplayName, exePath, opts.ConfigPath, workingDir)

		if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
			return fmt.Errorf("write systemd unit: %w", err)
		}
		if err := runCommand("systemctl", "daemon-reload"); err != nil {
			return err
		}
		if err := runCommand("systemctl", "enable", "--now", unitName); err != nil {
			return err
		}
		fmt.Printf("Service installed: %s\n", unitName)
		return nil
	case "uninstall":
		_ = runCommand("systemctl", "disable", "--now", unitName)
		if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove systemd unit: %w", err)
		}
		if err := runCommand("systemctl", "daemon-reload"); err != nil {
			return err
		}
		fmt.Printf("Service removed: %s\n", unitName)
		return nil
	case "start":
		return runCommand("systemctl", "start", unitName)
	case "stop":
		return runCommand("systemctl", "stop", unitName)
	case "restart":
		return runCommand("systemctl", "restart", unitName)
	case "status":
		return runCommand("systemctl", "--no-pager", "--full", "status", unitName)
	default:
		return fmt.Errorf("unsupported service action %q", opts.Action)
	}
}

func runDarwinServiceCommand(opts serviceCommandOptions, cfg *clientconfig.ClientConfig) error {
	label := strings.TrimSpace(opts.Name)
	plistPath := filepath.Join("/Library/LaunchDaemons", label+".plist")

	switch opts.Action {
	case "install":
		exePath, err := currentExecutablePath()
		if err != nil {
			return err
		}
		workingDir := resolveWorkingDir(cfg, opts.ConfigPath)
		logPath := opts.LogPath
		errorLogPath := darwinErrorLogPath(logPath)
		if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
			return fmt.Errorf("create log dir: %w", err)
		}

		plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>-c</string>
    <string>%s</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>WorkingDirectory</key>
  <string>%s</string>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, label, xmlEscape(exePath), xmlEscape(opts.ConfigPath), xmlEscape(workingDir), xmlEscape(logPath), xmlEscape(errorLogPath))

		if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
			return fmt.Errorf("write launchd plist: %w", err)
		}
		_ = runCommand("launchctl", "bootout", "system", plistPath)
		if err := runCommand("launchctl", "bootstrap", "system", plistPath); err != nil {
			return err
		}
		if err := runCommand("launchctl", "enable", "system/"+label); err != nil {
			return err
		}
		if err := runCommand("launchctl", "kickstart", "-k", "system/"+label); err != nil {
			return err
		}
		fmt.Printf("Service installed: %s\n", label)
		return nil
	case "uninstall":
		_ = runCommand("launchctl", "disable", "system/"+label)
		_ = runCommand("launchctl", "bootout", "system", plistPath)
		if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove launchd plist: %w", err)
		}
		fmt.Printf("Service removed: %s\n", label)
		return nil
	case "start":
		_ = runCommand("launchctl", "enable", "system/"+label)
		_ = runCommand("launchctl", "bootout", "system", plistPath)
		if err := runCommand("launchctl", "bootstrap", "system", plistPath); err != nil {
			return err
		}
		return runCommand("launchctl", "kickstart", "-k", "system/"+label)
	case "stop":
		_ = runCommand("launchctl", "disable", "system/"+label)
		return runCommand("launchctl", "bootout", "system", plistPath)
	case "restart":
		_ = runCommand("launchctl", "bootout", "system", plistPath)
		if err := runCommand("launchctl", "enable", "system/"+label); err != nil {
			return err
		}
		if err := runCommand("launchctl", "bootstrap", "system", plistPath); err != nil {
			return err
		}
		return runCommand("launchctl", "kickstart", "-k", "system/"+label)
	case "status":
		return runCommand("launchctl", "print", "system/"+label)
	default:
		return fmt.Errorf("unsupported service action %q", opts.Action)
	}
}

func linuxUnitName(name string) string {
	if strings.HasSuffix(name, ".service") {
		return name
	}
	return name + ".service"
}

func resolveWorkingDir(cfg *clientconfig.ClientConfig, configPath string) string {
	if dir := resolveServiceDataDir(cfg, configPath); dir != "" {
		return dir
	}
	if configPath != "" {
		return filepath.Dir(configPath)
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "/"
}

func darwinErrorLogPath(logPath string) string {
	if logPath == "" {
		return ""
	}
	ext := filepath.Ext(logPath)
	base := strings.TrimSuffix(logPath, ext)
	if base == "" {
		base = logPath
	}
	return base + "-error" + ext
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%s %s failed: %s", name, strings.Join(args, " "), strings.TrimSpace(stderr.String()))
		}
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}
	if stderr.Len() > 0 {
		fmt.Fprint(os.Stderr, stderr.String())
	}
	return nil
}
