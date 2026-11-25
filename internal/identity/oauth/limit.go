package oauth

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/rate"
)

// TokenPerIP is how many POST /oauth/token one client IP makes per minute
// (every grant, the agent token exchange included); DevicePollInterval is the
// shortest gap between polls of one pending device code (else slow_down).
const (
	TokenPerIP         = 60
	DevicePollInterval = 5 * time.Second
)

// limitToken counts POST /oauth/token per client IP before op: past
// TokenPerIP in a minute it answers 429 with Retry-After: 60, and a rate
// store error answers 500 (fail-closed). Other requests pass uncounted.
func limitToken(pool *pgxpool.Pool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/oauth/token" {
			next.ServeHTTP(w, r)
			return
		}
		ok, err := rate.Allow(r.Context(), pool, "token:ip:"+clientip.From(r.Context()).String(), TokenPerIP, time.Minute)
		switch {
		case err != nil:
			slog.ErrorContext(r.Context(), "oauth: token rate limit", "err", err)
			writeError(w, http.StatusInternalServerError, "server_error")
		case !ok:
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "too_many_requests")
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// writeError writes an OAuth error body {"error": code} with status.
func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + code + `"}`))
}
