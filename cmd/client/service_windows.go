//go:build windows

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	clientapp "github.com/gotunnel/internal/client/app"
	"golang.org/x/sys/windows/svc"
)

func runClient(opts runtimeOptions) error {
	if !opts.ServiceMode {
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
