// Package werbot is the entry point of the werbot binary.
package werbot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/werbot/werbot/internal/app"
)

// Main runs the werbot CLI until SIGINT or SIGTERM. An error with an
// ExitCode method ends the process with that code and no message; any other
// error prints "werbot: <err>" and exits 1.
func Main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	err := app.Run(ctx, os.Args[1:], os.Getenv, os.Stdout)
	stop()
	code, report := exitCode(err)
	if report {
		fmt.Fprintf(os.Stderr, "werbot: %v\n", err)
	}
	if code != 0 {
		os.Exit(code)
	}
}

// exitCode maps err to a process exit code; report says to print err.
func exitCode(err error) (code int, report bool) {
	if err == nil {
		return 0, false
	}
	var ec interface{ ExitCode() int }
	if errors.As(err, &ec) {
		return ec.ExitCode(), false
	}
	return 1, true
}
