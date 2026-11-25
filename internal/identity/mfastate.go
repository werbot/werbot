package identity

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/werbot/werbot/internal/platform/principal"
)

// mfaState is the caller's MFA situation, read with the credential in one
// statement and kept in the request context by Middleware: the
// organization's require_mfa, whether the user has a second factor (a TOTP
// secret or a passkey) and when the credential last passed one (a cookie
// session's mfa_at, the newest verification bound to a Bearer token's CLI
// session; nil: never).
type mfaState struct {
	RequireMFA string
	HasFactor  bool
	FreshAt    *time.Time
}

type mfaStateKey struct{}

func withMFAState(ctx context.Context, st mfaState) context.Context {
	return context.WithValue(ctx, mfaStateKey{}, st)
}

// mfaRequired reports whether require_mfa mode asks a second factor of p:
// "all", or "admins" for owners and admins.
func mfaRequired(mode string, p principal.Principal) bool {
	return mode == "all" || mode == "admins" && p.Manager()
}

// EnrollmentRequired reports whether the caller must enroll a second factor
// before anything else: the organization requires one of the caller's role
// and the user has none. An agent token answers for its sponsor.
func EnrollmentRequired(ctx context.Context) bool {
	st, _ := ctx.Value(mfaStateKey{}).(mfaState)
	p, _ := principal.From(ctx)
	return !st.HasFactor && mfaRequired(st.RequireMFA, p)
}

// mfaGate answers 403 "mfa enrollment required" to /api/ paths (except the
// OIDC login under /api/oidc/) while the caller must enroll a factor, and
// reports whether the request may go on. Connect procedures are gated by the
// interceptor; other paths serve the SPA, which leads to enrollment.
func mfaGate(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/api/oidc/") || !EnrollmentRequired(r.Context()) {
		return true
	}
	http.Error(w, "mfa enrollment required", http.StatusForbidden)
	return false
}
