//go:build !windows

package main

import "fmt"

func runClient(opts runtimeOptions) error {
	if opts.ServiceMode {
		return fmt.Errorf("-service is only supported on Windows")
	}
	return runConsoleClient(opts.AppConfig)
}
