//go:build linux

package vnet

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/tun"
)

const (
	tunName = "werbot0"
	// ngroupsMax is NGROUPS_MAX of linux/limits.h: setgroups refuses more groups.
	ngroupsMax = 65536
)

// openTUN creates werbot0; without root or CAP_NET_ADMIN the kernel refuses it.
func openTUN() (tun.Device, error) {
	dev, err := tun.CreateTUN(tunName, mtu)
	if errors.Is(err, os.ErrPermission) {
		exe, _ := os.Executable()
		return nil, fmt.Errorf(`--vnet needs CAP_NET_ADMIN: run with sudo or "sudo setcap cap_net_admin+ep %s"`, exe)
	}
	return dev, err
}

// configure gives dev the interface address with the prefix route and points
// systemd-resolved at the DNS address for ~suffix. The link settings vanish
// with the TUN, so the cleanup has nothing to do.
func configure(dev string, addr netip.Addr, prefix netip.Prefix, suffix string) (func() error, error) {
	steps := [][]string{
		{"ip", "addr", "add", netip.PrefixFrom(addr, prefix.Bits()).String(), "dev", dev},
		{"ip", "link", "set", dev, "up"},
	}
	for _, s := range steps {
		if err := command(s...); err != nil {
			return nil, err
		}
	}
	dns := DNSAddr(prefix).String()
	err := command("resolvectl", "dns", dev, dns)
	if err == nil {
		err = command("resolvectl", "domain", dev, "~"+suffix)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\nAdd \"nameserver %s\" before other nameservers in /etc/resolv.conf to resolve *.%s\n", err, dns, suffix)
	}
	return func() error { return nil }, nil
}

// procAttr passes CAP_NET_ADMIN on to ip and resolvectl as an ambient
// capability without root: a setcap binary's children do not inherit it otherwise.
func procAttr() *syscall.SysProcAttr {
	if os.Geteuid() == 0 {
		return nil
	}
	return &syscall.SysProcAttr{AmbientCaps: []uintptr{unix.CAP_NET_ADMIN}}
}
