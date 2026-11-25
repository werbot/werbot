package vnet

import (
	"net/netip"
	"os/user"
	"path/filepath"

	"github.com/werbot/werbot/internal/clientauth"
)

// Options configures Run. The DNS suffix (<org>.werbot) comes from
// GetSession inside Run, which calls the API only after dropping root.
type Options struct {
	Profile *clientauth.Profile
	Home    string             // profile home: SudoHome under sudo, else clientauth.Home
	Prefix  netip.Prefix       // DefaultPrefix or --vnet-prefix
	Open    func(string) error // opens an MFA URL in a browser; nil prints it only (under sudo)
}

// SudoHome is <home of SUDO_USER>/.werbot when the command runs under sudo
// and WERBOT_HOME is empty: sudo resets HOME, the profile stays the user's.
func SudoHome(getenv func(string) string) (string, bool) {
	name := getenv("SUDO_USER")
	if name == "" || getenv("WERBOT_HOME") != "" {
		return "", false
	}
	u, err := user.Lookup(name)
	if err != nil || u.HomeDir == "" {
		return "", false
	}
	return filepath.Join(u.HomeDir, ".werbot"), true
}
