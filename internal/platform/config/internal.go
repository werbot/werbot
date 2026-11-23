package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// loadInternal reads WERBOT_INTERNAL_ADDR (default ":8444", port 1-65535) and
// WERBOT_INTERNAL_URL (default "https://<os.Hostname()>:<port>", https only,
// no trailing "/"): the replica-to-replica mTLS listener and its address.
func loadInternal(getenv func(string) string) (addr, internalURL string, err error) {
	addr = or(getenv("WERBOT_INTERNAL_ADDR"), ":8444")
	_, port, err := net.SplitHostPort(addr)
	if n, perr := strconv.Atoi(port); err != nil || perr != nil || n < 1 || n > 65535 {
		return "", "", fmt.Errorf("WERBOT_INTERNAL_ADDR: want [host]:port with port 1-65535, got %q", addr)
	}
	v := getenv("WERBOT_INTERNAL_URL")
	if v == "" {
		host, err := os.Hostname()
		if err != nil {
			return "", "", fmt.Errorf("WERBOT_INTERNAL_URL: empty and no hostname: %w", err)
		}
		return addr, "https://" + net.JoinHostPort(host, port), nil
	}
	if u, err := url.Parse(v); err != nil || u.Scheme != "https" || u.Host == "" {
		return "", "", fmt.Errorf("WERBOT_INTERNAL_URL: want https://host, got %q", v)
	}
	return addr, strings.TrimSuffix(v, "/"), nil
}
