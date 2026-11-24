//go:build darwin

package vnet

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"syscall"

	"golang.zx2c4.com/wireguard/tun"
)

// resolverDir holds the per-domain resolver files of macOS.
const resolverDir = "/etc/resolver"

// ngroupsMax is NGROUPS_MAX of macOS: setgroups refuses more groups.
const ngroupsMax = 16

// openTUN creates the next free utunN; macOS allows it only to root.
func openTUN() (tun.Device, error) {
	if os.Geteuid() != 0 {
		return nil, errors.New(`--vnet needs root: run "sudo werbot connect --vnet"`)
	}
	return tun.CreateTUN("utun", mtu)
}

// configure gives dev the interface address and the prefix route, and writes
// /etc/resolver/<suffix> so macOS asks the DNS address for *.suffix. The
// cleanup removes the file; the address and route vanish with the utun.
func configure(dev string, addr netip.Addr, prefix netip.Prefix, suffix string) (func() error, error) {
	steps := [][]string{
		{"ifconfig", dev, "inet", addr.String(), addr.String(), "up"},
		{"route", "-q", "-n", "add", "-inet", prefix.Masked().String(), "-interface", dev},
	}
	for _, s := range steps {
		if err := command(s...); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(resolverDir, 0o755); err != nil { //nolint:gosec // a system directory every user reads
		return nil, err
	}
	// suffix is an OrgLabel plus ".werbot": no path separators
	file := filepath.Join(resolverDir, suffix)
	if err := os.WriteFile(file, []byte("nameserver "+DNSAddr(prefix).String()+"\n"), 0o644); err != nil { //nolint:gosec // resolver files are world-readable
		return nil, err
	}
	return func() error { return os.Remove(file) }, nil
}

func procAttr() *syscall.SysProcAttr { return nil }
