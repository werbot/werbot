//go:build !linux && !darwin

package vnet

import (
	"context"
	"fmt"
	"runtime"
)

// Run is not supported here: neither the TUN driver nor the stack is linked.
func Run(context.Context, Options) error {
	return fmt.Errorf("--vnet is not supported on %s", runtime.GOOS)
}
