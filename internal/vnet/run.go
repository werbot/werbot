//go:build linux || darwin

package vnet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/crypto/ssh"
	"golang.zx2c4.com/wireguard/tun"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
)

// system is what Run takes from the OS; tests replace it.
type system struct {
	openTUN   func() (tun.Device, error) // checks the privileges first
	drop      func() (regain func() error, err error)
	configure func(dev string, addr netip.Addr, prefix netip.Prefix, suffix string) (cleanup func() error, err error)
	dialer    func(Options) Dialer
	out       io.Writer // the name → address lines
}

// refreshEvery is how often the resource list is fetched again.
var refreshEvery = 60 * time.Second

// Run gives every accessible resource an address and a name under
// <org>.werbot until ctx ends. Order (Ruling G8): open the TUN as root, drop
// to the sudo user for the API calls, regain root for the address, route and
// DNS, drop again to serve, and on exit stop the dialer, then regain root for
// the cleanup and close the TUN.
func Run(ctx context.Context, opts Options) error {
	return run(ctx, opts, system{
		openTUN:   openTUN,
		drop:      func() (func() error, error) { return dropPrivileges(os.Getenv) },
		configure: configure,
		dialer:    gatewayDialerOf,
		out:       os.Stdout,
	})
}

// gatewayDialerOf dials as the profile's login with its certificate; opts.Open
// opens MFA URLs in a browser (nil prints them only).
func gatewayDialerOf(opts Options) Dialer {
	p, home := opts.Profile, opts.Home
	return newGatewayDialer(p.Login, func(ctx context.Context) (ssh.Signer, error) { return p.Signer(ctx, home) }, opts.Open)
}

func run(ctx context.Context, opts Options, sys system) error {
	if _, err := base(opts.Prefix); err != nil {
		return err
	}
	dev, err := sys.openTUN()
	if err != nil {
		return err
	}
	suffix, rs, cleanup, err := setup(ctx, opts, sys, dev)
	if err != nil {
		return errors.Join(err, dev.Close())
	}
	regain, err := sys.drop()
	if err != nil {
		return errors.Join(err, cleanup(), dev.Close())
	}
	d := sys.dialer(opts)
	e, err := newEngine(dev, opts.Prefix, suffix, d)
	if err != nil {
		_ = d.Close()
		return errors.Join(err, regain(), cleanup(), dev.Close())
	}
	err = serve(ctx, e, opts, suffix, rs, sys.out)
	serr := e.stop() // nothing dials or writes the profile as root
	rerr := regain() // root for the OS cleanup, then close the TUN
	cerr := cleanup()
	return errors.Join(err, serr, rerr, cerr, e.Close())
}

// setup calls the API with root dropped, then sets up the interface and the
// OS DNS as root. A failed regain stops it before any OS change.
func setup(ctx context.Context, opts Options, sys system, dev tun.Device) (string, []Resource, func() error, error) {
	name, err := dev.Name()
	if err != nil {
		return "", nil, nil, err
	}
	regain, err := sys.drop()
	if err != nil {
		return "", nil, nil, err
	}
	suffix, rs, err := fetch(ctx, opts)
	if rerr := regain(); rerr != nil {
		return "", nil, nil, rerr
	}
	if err != nil {
		return "", nil, nil, err
	}
	cleanup, err := sys.configure(name, IfaceAddr(opts.Prefix), opts.Prefix, suffix)
	return suffix, rs, cleanup, err
}

// fetch is the DNS suffix of the organization and the resources to map.
func fetch(ctx context.Context, opts Options) (string, []Resource, error) {
	p := opts.Profile
	sess, err := werbotv1connect.NewAuthServiceClient(p.HTTPClient(), p.Server).
		GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
	if err != nil {
		return "", nil, err
	}
	rs, err := list(ctx, opts)
	return OrgLabel(sess.Msg.GetOrganization().GetName(), p.OrgID) + ".werbot", rs, err
}

func list(ctx context.Context, opts Options) ([]Resource, error) {
	p := opts.Profile
	res, err := werbotv1connect.NewResourceServiceClient(p.HTTPClient(), p.Server).
		ListAccessibleResources(ctx, connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	if err != nil {
		return nil, err
	}
	return resourcesOf(res.Msg.GetResources()), nil
}

// serve maps rs, then fetches the list every refreshEvery until ctx ends.
// A failed refresh keeps the last list; the lines are printed when they change.
func serve(ctx context.Context, e *engine, opts Options, suffix string, rs []Resource, out io.Writer) error {
	as, err := e.SetResources(rs)
	if err != nil {
		return err
	}
	printAssignments(out, suffix, as)
	tick := time.NewTicker(refreshEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
		var next []Assignment
		rs, err := list(ctx, opts)
		if err == nil {
			next, err = e.SetResources(rs)
		}
		switch {
		case err != nil && ctx.Err() == nil:
			_, _ = fmt.Fprintf(os.Stderr, "vnet refresh: %v\n", err)
		case err == nil && !slices.Equal(as, next):
			as = next
			printAssignments(out, suffix, as)
		}
	}
}

func printAssignments(out io.Writer, suffix string, as []Assignment) {
	for _, a := range as {
		_, _ = fmt.Fprintf(out, "%s.%s → %s (%s)\n", strings.ToLower(a.Name), suffix, a.Addr, a.Proto)
	}
}

// command runs a network tool of the OS setup.
func command(args ...string) error {
	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec // fixed tools; the suffix is an OrgLabel
	cmd.SysProcAttr = procAttr()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
