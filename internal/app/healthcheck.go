package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"
)

const healthcheckTimeout = 3 * time.Second

// healthcheck probes GET /healthz on httpAddr (WERBOT_HTTP_ADDR form). An empty
// or unspecified host is probed as 127.0.0.1. Liveness only: no database check.
func healthcheck(ctx context.Context, httpAddr string) error {
	host, port, err := net.SplitHostPort(httpAddr)
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	if addr, perr := netip.ParseAddr(host); host == "" || (perr == nil && addr.IsUnspecified()) {
		host = "127.0.0.1"
	}
	ctx, cancel := context.WithTimeout(ctx, healthcheckTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: unexpected status %d", resp.StatusCode)
	}
	return nil
}
