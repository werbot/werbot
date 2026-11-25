package identity

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// ipCtx is a request context whose client IP (clientip.From) is ip.
func ipCtx(ip string) context.Context {
	ctx := context.Background()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = ip + ":40000"
	clientip.Middleware(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
	})).ServeHTTP(httptest.NewRecorder(), r)
	return ctx
}

// TestLoginRateLimitPerIPEmail: every attempt of one IP and email counts; past
// LoginPerIPEmail in a minute even the right password is refused, while the
// same email from another IP still reaches the password check.
func TestLoginRateLimitPerIPEmail(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	dbtest.FreshRateWindow()
	a := ipCtx("203.0.113.10")
	for i := range LoginPerIPEmail {
		_, err := svc.Login(a, "alice@example.com", "wrong horse battery", "test")
		require.ErrorIs(t, err, apperr.ErrUnauthenticated, "attempt %d", i+1)
	}
	_, err := svc.Login(a, " Alice@Example.com", testPassword, "test")
	require.ErrorIs(t, err, apperr.ErrResourceExhausted, "the normalized email shares the count")
	assert.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(apperr.ToConnect(err)))

	b := ipCtx("203.0.113.11")
	_, err = svc.Login(b, "alice@example.com", "wrong horse battery", "test")
	require.ErrorIs(t, err, apperr.ErrUnauthenticated, "another IP: the password is checked")
	_, err = svc.Login(b, "alice@example.com", testPassword, "test")
	require.NoError(t, err)
}

// TestLoginRateLimitPerIP: past LoginPerIP attempts from one IP in a minute,
// any email is refused.
func TestLoginRateLimitPerIP(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	dbtest.FreshRateWindow()
	a := ipCtx("203.0.113.20")
	var wg sync.WaitGroup
	for i := range LoginPerIP {
		wg.Go(func() {
			_, err := svc.Login(a, fmt.Sprintf("user%d@example.com", i), "wrong horse battery", "test")
			assert.ErrorIs(t, err, apperr.ErrUnauthenticated)
		})
	}
	wg.Wait()
	_, err := svc.Login(a, "alice@example.com", testPassword, "test")
	require.ErrorIs(t, err, apperr.ErrResourceExhausted)
	_, err = svc.Login(ipCtx("203.0.113.21"), "alice@example.com", testPassword, "test")
	require.NoError(t, err, "another IP has its own count")
}

// TestResetRateLimit: past ResetPerEmail requests in an hour the request is
// refused; a known and an unknown email answer alike.
func TestResetRateLimit(t *testing.T) {
	svc, pool, q := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	ctx := context.Background()
	for _, email := range []string{"alice@example.com", "nobody@example.com"} {
		for i := range ResetPerEmail {
			require.NoError(t, svc.RequestPasswordReset(ctx, email), "%s request %d", email, i+1)
		}
		err := svc.RequestPasswordReset(ctx, email)
		require.ErrorIs(t, err, apperr.ErrResourceExhausted, email)
		assert.Equal(t, "too many requests", err.Error())
	}
	assert.Len(t, q.Of("password_reset"), ResetPerEmail, "only alice's first requests send email")
	require.ErrorIs(t, svc.RequestPasswordReset(ctx, " ALICE@example.com "), apperr.ErrResourceExhausted, "normalized email")
}

// TestRateAllowStoreErrorDenies: when the rate store fails, Login with the
// right password and a password reset of a known email are refused
// (fail-closed): no session, no email.
func TestRateAllowStoreErrorDenies(t *testing.T) {
	svc, pool, q := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	ctx := context.Background()
	_, err := pool.Exec(ctx, "DROP TABLE rate_limit")
	require.NoError(t, err)

	_, err = svc.Login(ipCtx("203.0.113.30"), "alice@example.com", testPassword, "test")
	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(apperr.ToConnect(err)))
	require.Error(t, svc.RequestPasswordReset(ctx, "alice@example.com"))
	assert.Empty(t, q.Sent())
	var sessions int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM user_session").Scan(&sessions))
	assert.Zero(t, sessions)
}
