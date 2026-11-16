// Package werbot is the entry point of the werbot binary.
package werbot

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/werbot/werbot/internal/app"
)

// Main runs the werbot CLI until SIGINT or SIGTERM and exits 1 on error.
func Main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	err := app.Run(ctx, os.Args[1:], os.Getenv, os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "werbot: %v\n", err)
		os.Exit(1)
	}
}
