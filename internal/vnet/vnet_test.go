//go:build linux || darwin

package vnet

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/tun/netstack"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
)

func TestSudoHome(t *testing.T) {
	u, err := user.Current()
	require.NoError(t, err)
	env := map[string]string{"SUDO_USER": u.Username}
	getenv := func(k string) string { return env[k] }

	home, ok := SudoHome(getenv)
	assert.True(t, ok)
	assert.Equal(t, filepath.Join(u.HomeDir, ".werbot"), home)

	env["WERBOT_HOME"] = t.TempDir()
	_, ok = SudoHome(getenv)
	assert.False(t, ok, "WERBOT_HOME wins")

	clear(env)
	_, ok = SudoHome(getenv)
	assert.False(t, ok, "not under sudo")

	env["SUDO_USER"] = "no-such-user-" + uuid.NewString()[:8]
	_, ok = SudoHome(getenv)
	assert.False(t, ok, "unknown user")
}

func TestRunRequiresPrivileges(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("needs a user without root")
	}
	err := Run(t.Context(), Options{Prefix: testPrefix})
	if runtime.GOOS == "linux" {
		require.ErrorContains(t, err, "CAP_NET_ADMIN")
		return
	}
	require.EqualError(t, err, `--vnet needs root: run "sudo werbot connect --vnet"`)
}

// events is the order in which Run reached the OS and the API.
type events struct {
	mu   sync.Mutex
	list []string
}

func (e *events) add(s string) { e.mu.Lock(); e.list = append(e.list, s); e.mu.Unlock() }

func (e *events) get() []string { e.mu.Lock(); defer e.mu.Unlock(); return slices.Clone(e.list) }

// fakeAPI answers GetSession and ListAccessibleResources and logs each call.
type fakeAPI struct {
	werbotv1connect.UnimplementedAuthServiceHandler
	werbotv1connect.UnimplementedResourceServiceHandler
	ev        *events
	mu        sync.Mutex
	resources []*werbotv1.AccessibleResource
}

func (f *fakeAPI) GetSession(context.Context, *connect.Request[werbotv1.GetSessionRequest]) (*connect.Response[werbotv1.GetSessionResponse], error) {
	f.ev.add("session")
	return connect.NewResponse(&werbotv1.GetSessionResponse{Organization: &werbotv1.Organization{Name: "Acme Corp"}}), nil
}

func (f *fakeAPI) ListAccessibleResources(context.Context, *connect.Request[werbotv1.ListAccessibleResourcesRequest]) (*connect.Response[werbotv1.ListAccessibleResourcesResponse], error) {
	f.ev.add("resources")
	f.mu.Lock()
	defer f.mu.Unlock()
	return connect.NewResponse(&werbotv1.ListAccessibleResourcesResponse{Resources: slices.Clone(f.resources)}), nil
}

func (f *fakeAPI) add(r *werbotv1.AccessibleResource) {
	f.mu.Lock()
	f.resources = append(f.resources, r)
	f.mu.Unlock()
}

func accessibleRes(name string, allowed bool, kinds ...string) *werbotv1.AccessibleResource {
	return &werbotv1.AccessibleResource{Id: uuid.NewString(), Name: name, Allowed: allowed, Kinds: kinds, SshAddress: "gw.example.com:2222"}
}

// syncBuffer is the output of Run, read while Run writes it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

// testRun is a fake API with four resources and the options of a run against it.
func testRun(t *testing.T) (*fakeAPI, *events, Options) {
	t.Helper()
	ev := &events{}
	api := &fakeAPI{ev: ev, resources: []*werbotv1.AccessibleResource{
		accessibleRes("pg", true, "stream"), accessibleRes("dns", true, "datagram"),
		accessibleRes("ops", true, "terminal"), accessibleRes("vault", false, "stream"),
	}}
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewAuthServiceHandler(api))
	mux.Handle(werbotv1connect.NewResourceServiceHandler(api))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	p := &clientauth.Profile{Server: srv.URL, Login: "alice", OrgID: uuid.New(), AccessToken: "a", AccessExpiry: time.Now().Add(time.Hour)}
	return api, ev, Options{Profile: p, Home: t.TempDir(), Prefix: testPrefix}
}

// fakeSystem logs every OS step into ev; failRegain makes the first regain fail.
func fakeSystem(t *testing.T, ev *events, out *syncBuffer, failRegain bool) system {
	t.Helper()
	regains := 0
	return system{
		openTUN: func() (tun.Device, error) {
			ev.add("tun")
			dev, _, err := netstack.CreateNetTUN([]netip.Addr{IfaceAddr(testPrefix)}, []netip.Addr{DNSAddr(testPrefix)}, 1500)
			return closingTUN{dev, ev}, err
		},
		drop: func() (func() error, error) {
			ev.add("drop")
			return func() error {
				ev.add("regain")
				if regains++; failRegain && regains == 1 {
					return errors.New("seteuid: operation not permitted")
				}
				return nil
			}, nil
		},
		configure: func(_ string, addr netip.Addr, prefix netip.Prefix, suffix string) (func() error, error) {
			ev.add("configure " + addr.String() + " " + prefix.String() + " " + suffix)
			return func() error { ev.add("cleanup"); return nil }, nil
		},
		dialer: func(Options) Dialer { return closingDialer{newFakeDialer(t), ev} },
		out:    out,
	}
}

// closingTUN and closingDialer add "tun close" and "dialer close" to ev on Close.
type closingTUN struct {
	tun.Device
	ev *events
}

func (c closingTUN) Close() error { c.ev.add("tun close"); return c.Device.Close() }

type closingDialer struct {
	Dialer
	ev *events
}

func (c closingDialer) Close() error { c.ev.add("dialer close"); return c.Dialer.Close() }

// Run opens the TUN as root, calls the API with privileges dropped, sets up
// the OS as root, serves and refreshes with privileges dropped, and on exit
// closes the dialer, so nothing dials or renews the certificate as root, then
// regains root for the OS cleanup and closes the TUN last (Ruling G8).
func TestRunOrder(t *testing.T) {
	old := refreshEvery
	refreshEvery = 20 * time.Millisecond
	t.Cleanup(func() { refreshEvery = old })

	out := &syncBuffer{}
	api, ev, opts := testRun(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- run(ctx, opts, fakeSystem(t, ev, out, false)) }()

	require.Eventually(t, func() bool { return strings.Contains(out.String(), "pg.acme-corp.werbot → 100.") }, 5*time.Second, 10*time.Millisecond)
	// two refreshes of the unchanged list before it changes
	require.Eventually(t, func() bool { return len(ev.get()) >= 9 }, 5*time.Second, 10*time.Millisecond)
	api.add(accessibleRes("cache", true, "stream"))
	require.Eventually(t, func() bool { return strings.Contains(out.String(), "cache.acme-corp.werbot → 100.") }, 5*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err, "Ctrl+C ends with code 0")
	case <-time.After(5 * time.Second):
		t.Fatal("run did not stop")
	}

	text := out.String()
	assert.Contains(t, text, "pg.acme-corp.werbot → ")
	assert.Contains(t, text, " (tcp)\n")
	assert.Regexp(t, `dns\.acme-corp\.werbot → 100\.\d+\.\d+\.\d+ \(udp\)`, text)
	assert.NotContains(t, text, "ops.", "terminal-only resources get no address")
	assert.NotContains(t, text, "vault.", "resources not allowed get no address")
	assert.Equal(t, 2, strings.Count(text, "pg.acme-corp.werbot"), "the list is printed at start and when it changes, not on each refresh")

	got := ev.get()
	require.GreaterOrEqual(t, len(got), 9)
	assert.Equal(t, []string{"tun", "drop", "session", "resources", "regain", "configure 100.64.0.1 100.64.0.0/10 acme-corp.werbot", "drop"}, got[:7])
	// only refreshes run after the second drop; a refresh in flight at cancel
	// may reach the fake server at any later step
	exit := slices.DeleteFunc(got[7:], func(e string) bool { return e == "resources" })
	assert.Equal(t, []string{"dialer close", "regain", "cleanup", "tun close"}, exit)
}

// A regain that fails after the API calls stops Run before the OS setup.
func TestRunStopsWhenRegainFails(t *testing.T) {
	_, ev, opts := testRun(t)
	err := run(ctxT(t, 5*time.Second), opts, fakeSystem(t, ev, &syncBuffer{}, true)) // ends even when it would serve
	require.ErrorContains(t, err, "seteuid: operation not permitted")
	assert.Equal(t, []string{"tun", "drop", "session", "resources", "regain", "tun close"}, ev.get())
}

func TestRunRejectsPrefix(t *testing.T) {
	ev := &events{}
	err := run(t.Context(), Options{Prefix: netip.MustParsePrefix("100.64.0.0/30")}, fakeSystem(t, ev, &syncBuffer{}, false))
	require.ErrorContains(t, err, "at least a /29")
	assert.Empty(t, ev.get(), "nothing is opened")
}
