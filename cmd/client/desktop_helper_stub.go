//go:build !windows

package main

import "fmt"

func runDesktopHelperCLI(_ []string) error {
	return fmt.Errorf("desktop helper is only supported on windows")
}
