//go:build windows

package main

import (
	"context"
	"flag"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/gotunnel/internal/client/desktop"
)

func runDesktopHelperCLI(args []string) error {
	fs := flag.NewFlagSet("desktop-agent", flag.ContinueOnError)
	dataDir := fs.String("data-dir", "", "client data directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dataDir == "" {
		return fmt.Errorf("-data-dir is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return desktop.RunHelper(ctx, *dataDir, 0)
}
