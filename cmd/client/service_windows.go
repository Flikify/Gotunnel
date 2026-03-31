//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"

	clientapp "github.com/gotunnel/internal/client/app"
	clientconfig "github.com/gotunnel/internal/client/config"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func runClient(opts runtimeOptions) error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return fmt.Errorf("check service mode: %w", err)
	}
	if !isService {
		return runConsoleClient(opts.AppConfig)
	}
	return runWindowsService(opts)
}

func runWindowsService(opts runtimeOptions) error {
	logPath := opts.ServiceLogPath
	if logPath == "" && opts.AppConfig.DataDir != "" {
		logPath = filepath.Join(opts.AppConfig.DataDir, "service.log")
	}
	if err := configureWindowsServiceLog(logPath); err != nil {
		return err
	}

	log.Printf("[Service] starting %s", opts.ServiceName)
	return svc.Run(opts.ServiceName, &goTunnelService{
		name: opts.ServiceName,
		cfg:  opts.AppConfig,
	})
}

func runServiceCommand(opts serviceCommandOptions, _ *clientconfig.ClientConfig) error {
	switch opts.Action {
	case "install":
		return installWindowsService(opts)
	case "uninstall":
		return uninstallWindowsService(opts.Name)
	case "start":
		return startWindowsService(opts.Name)
	case "stop":
		return stopWindowsService(opts.Name)
	case "restart":
		if err := stopWindowsService(opts.Name); err != nil {
			return err
		}
		return startWindowsService(opts.Name)
	case "status":
		return statusWindowsService(opts.Name)
	default:
		return fmt.Errorf("unsupported service action %q", opts.Action)
	}
}

type goTunnelService struct {
	name string
	cfg  clientapp.Config
}

func (s *goTunnelService) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}

	app := clientapp.NewService()
	app.Configure(s.cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runErr := make(chan error, 1)
	go func() {
		runErr <- app.RunContext(ctx)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: accepted}
	log.Printf("[Service] %s is running", s.name)

	for {
		select {
		case req := <-requests:
			switch req.Cmd {
			case svc.Interrogate:
				changes <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				log.Printf("[Service] received stop request")
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				if err := <-runErr; err != nil {
					log.Printf("[Service] runtime stopped with error during shutdown: %v", err)
				}
				return false, 0
			default:
			}
		case err := <-runErr:
			if err != nil {
				log.Printf("[Service] runtime exited with error: %v", err)
			} else {
				log.Printf("[Service] runtime exited cleanly")
			}
			changes <- svc.Status{State: svc.StopPending}
			return false, 0
		}
	}
}

func configureWindowsServiceLog(path string) error {
	if path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create service log dir: %w", err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open service log file: %w", err)
	}

	log.SetOutput(file)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)
	log.Printf("[Service] bootstrap logging redirected to %s", path)
	return nil
}

func installWindowsService(opts serviceCommandOptions) error {
	exePath, err := currentExecutablePath()
	if err != nil {
		return err
	}
	if opts.LogPath != "" {
		if err := os.MkdirAll(filepath.Dir(opts.LogPath), 0755); err != nil {
			return fmt.Errorf("create service log dir: %w", err)
		}
	}

	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer manager.Disconnect()

	service, existing, err := openOrCreateWindowsService(manager, exePath, opts)
	if err != nil {
		return err
	}
	defer service.Close()

	if existing {
		if err := stopWindowsServiceHandle(service); err != nil {
			return err
		}
		if err := service.UpdateConfig(windowsServiceConfig(exePath, opts)); err != nil {
			return fmt.Errorf("update service config: %w", err)
		}
	}

	if err := service.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
	}, 86400); err != nil {
		return fmt.Errorf("set recovery actions: %w", err)
	}
	if err := service.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		return fmt.Errorf("set recovery flags: %w", err)
	}

	if err := service.Start(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return fmt.Errorf("start service: %w", err)
	}
	fmt.Printf("Service installed: %s\n", opts.Name)
	return nil
}

func openOrCreateWindowsService(manager *mgr.Mgr, exePath string, opts serviceCommandOptions) (*mgr.Service, bool, error) {
	service, err := manager.OpenService(opts.Name)
	if err == nil {
		return service, true, nil
	}
	if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return nil, false, fmt.Errorf("open service: %w", err)
	}

	service, err = manager.CreateService(
		opts.Name,
		exePath,
		windowsServiceConfig(exePath, opts),
		windowsServiceArgs(opts)...,
	)
	if err != nil {
		return nil, false, fmt.Errorf("create service: %w", err)
	}
	return service, false, nil
}

func windowsServiceConfig(exePath string, opts serviceCommandOptions) mgr.Config {
	return mgr.Config{
		DisplayName:    opts.DisplayName,
		Description:    "GoTunnel client tunnel service managed by the client binary.",
		StartType:      mgr.StartAutomatic,
		ErrorControl:   mgr.ErrorNormal,
		BinaryPathName: buildWindowsServiceCommandLine(exePath, windowsServiceArgs(opts)),
	}
}

func windowsServiceArgs(opts serviceCommandOptions) []string {
	return []string{"-c", opts.ConfigPath}
}

func buildWindowsServiceCommandLine(exePath string, args []string) string {
	commandLine := syscall.EscapeArg(exePath)
	for _, arg := range args {
		commandLine += " " + syscall.EscapeArg(arg)
	}
	return commandLine
}

func uninstallWindowsService(name string) error {
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(name)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			fmt.Printf("Service already removed: %s\n", name)
			return nil
		}
		return fmt.Errorf("open service: %w", err)
	}
	defer service.Close()

	if err := stopWindowsServiceHandle(service); err != nil {
		return err
	}
	if err := service.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	fmt.Printf("Service removed: %s\n", name)
	return nil
}

func startWindowsService(name string) error {
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(name)
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer service.Close()

	if err := service.Start(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return fmt.Errorf("start service: %w", err)
	}
	fmt.Printf("Service started: %s\n", name)
	return nil
}

func stopWindowsService(name string) error {
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(name)
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer service.Close()

	if err := stopWindowsServiceHandle(service); err != nil {
		return err
	}
	fmt.Printf("Service stopped: %s\n", name)
	return nil
}

func stopWindowsServiceHandle(service *mgr.Service) error {
	status, err := service.Query()
	if err != nil {
		return fmt.Errorf("query service status: %w", err)
	}
	if status.State == svc.Stopped {
		return nil
	}

	_, err = service.Control(svc.Stop)
	if err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
		return fmt.Errorf("send stop control: %w", err)
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		status, err = service.Query()
		if err != nil {
			return fmt.Errorf("query service status: %w", err)
		}
		if status.State == svc.Stopped {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("service did not stop within timeout")
}

func statusWindowsService(name string) error {
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(name)
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer service.Close()

	status, err := service.Query()
	if err != nil {
		return fmt.Errorf("query service status: %w", err)
	}
	config, err := service.Config()
	if err != nil {
		return fmt.Errorf("query service config: %w", err)
	}

	fmt.Printf("Service name: %s\n", name)
	fmt.Printf("Display name: %s\n", config.DisplayName)
	fmt.Printf("State: %s\n", windowsServiceStateString(status.State))
	fmt.Printf("Binary path: %s\n", config.BinaryPathName)
	fmt.Printf("Start type: %d\n", config.StartType)
	fmt.Printf("PID: %d\n", status.ProcessId)
	return nil
}

func windowsServiceStateString(state svc.State) string {
	switch state {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "start-pending"
	case svc.StopPending:
		return "stop-pending"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continue-pending"
	case svc.PausePending:
		return "pause-pending"
	case svc.Paused:
		return "paused"
	default:
		return fmt.Sprintf("unknown(%d)", state)
	}
}
