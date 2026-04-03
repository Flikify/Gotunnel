//go:build !windows

package desktop

import (
	"context"
	"fmt"

	"github.com/gotunnel/internal/client/tunnel"
)

func RunHelper(_ context.Context, _ string, _ uint32) error {
	return fmt.Errorf("desktop helper is only supported on windows")
}

func NewServiceRemoteOpsProxy(_ string) tunnel.RemoteOpsProxy {
	return nil
}
