//go:build linux || darwin

package vnet

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"
)

// dropPrivileges switches the effective ids of a root process started by sudo
// to SUDO_UID and SUDO_GID with the user's supplementary groups, so profile
// files it writes stay the user's. regain switches back to root for the OS
// setup and cleanup. Without SUDO_UID, or when not root, both are no-ops.
// The saved uid stays 0, so a compromised process could regain root (R7):
// setcap on Linux avoids root altogether.
func dropPrivileges(getenv func(string) string) (regain func() error, err error) {
	noop := func() error { return nil }
	if getenv("SUDO_UID") == "" {
		return noop, nil
	}
	uid, err := strconv.Atoi(getenv("SUDO_UID"))
	if err != nil {
		return nil, fmt.Errorf("SUDO_UID: %w", err)
	}
	gid, err := strconv.Atoi(getenv("SUDO_GID"))
	if err != nil {
		return nil, fmt.Errorf("SUDO_GID: %w", err)
	}
	if os.Geteuid() != 0 {
		return noop, nil
	}
	rootGroups, err := syscall.Getgroups()
	if err != nil {
		return nil, fmt.Errorf("getgroups: %w", err)
	}
	if err := setIDs(groupsOf(getenv("SUDO_UID"), gid), gid, uid); err != nil {
		return nil, fmt.Errorf("drop root: %w", err)
	}
	return func() error {
		if err := syscall.Seteuid(0); err != nil {
			return fmt.Errorf("regain root: %w", err)
		}
		return setIDs(rootGroups, 0, 0)
	}, nil
}

// setIDs sets the groups, then the egid, then the euid: the first two need root.
func setIDs(groups []int, gid, uid int) error {
	if err := syscall.Setgroups(groups); err != nil {
		return fmt.Errorf("setgroups: %w", err)
	}
	if err := syscall.Setegid(gid); err != nil {
		return fmt.Errorf("setegid: %w", err)
	}
	if err := syscall.Seteuid(uid); err != nil {
		return fmt.Errorf("seteuid: %w", err)
	}
	return nil
}

// groupsOf lists the groups of the user uid, gid first and at most
// ngroupsMax of them; only gid when they are unknown.
func groupsOf(uid string, gid int) []int {
	out := []int{gid}
	u, err := user.LookupId(uid)
	if err != nil {
		return out
	}
	ids, err := u.GroupIds()
	if err != nil {
		return out
	}
	for _, s := range ids {
		if g, err := strconv.Atoi(s); err == nil && g != gid {
			out = append(out, g)
		}
	}
	return out[:min(len(out), ngroupsMax)]
}
