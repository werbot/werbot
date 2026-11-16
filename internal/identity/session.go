package identity

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

const (
	sessionMaxAge = 7 * 24 * 60 * 60 // seconds; the SQL side uses interval '7 days'
	maxUserAgent  = 512              // bytes stored in user_session.user_agent
)

// createSession stores a new session for userID in orgID and returns its token.
func (s *Service) createSession(ctx context.Context, st Store, userID, orgID uuid.UUID, userAgent string) (string, error) {
	raw, hash := newSecret()
	var ip *netip.Addr
	if a := clientip.From(ctx); a.IsValid() {
		ip = &a
	}
	if len(userAgent) > maxUserAgent {
		userAgent = userAgent[:maxUserAgent]
	}
	err := st.CreateSession(ctx, dbq.CreateSessionParams{
		TokenHash: hash, UserID: userID, OrgID: orgID, Ip: ip,
		UserAgent: strings.ToValidUTF8(userAgent, ""), // the cut or the client may leave invalid UTF-8
	})
	if err != nil {
		return "", err
	}
	return raw, nil
}

// AuthenticateSession resolves a session token to its principal; an unknown
// or expired token, or a removed membership, gives ErrUnauthenticated.
func (s *Service) AuthenticateSession(ctx context.Context, rawToken string) (principal.Principal, error) {
	p, _, err := s.authenticate(ctx, rawToken)
	return p, err
}

// authenticate is AuthenticateSession that also reports whether the session
// was extended (last seen over a minute ago), so the cookie is set again.
func (s *Service) authenticate(ctx context.Context, rawToken string) (principal.Principal, bool, error) {
	var (
		p        principal.Principal
		extended bool
	)
	err := dbx.System(ctx, s.pool, dbx.ReasonSession, func(tx pgx.Tx) error {
		st := newStore(tx)
		row, err := st.GetSessionByToken(ctx, hashSecret(rawToken))
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrUnauthenticated
		}
		if err != nil {
			return err
		}
		p = principal.Principal{UserID: row.UserID, OrgID: row.OrgID, Role: row.Role, SessionID: row.ID}
		if !row.Stale {
			return nil
		}
		extended = true
		return st.ExtendSession(ctx, dbq.ExtendSessionParams{ID: row.ID, OrgID: row.OrgID})
	})
	if err != nil {
		return principal.Principal{}, false, err
	}
	return p, extended, nil
}

// CurrentSession returns the caller's user, organization and role.
func (s *Service) CurrentSession(ctx context.Context) (SessionInfo, error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return SessionInfo{}, err
	}
	var info SessionInfo
	err = dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		r, err := newStore(tx).GetSessionInfo(ctx, dbq.GetSessionInfoParams{OrgID: p.OrgID, UserID: p.UserID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrUnauthenticated
		}
		if err != nil {
			return err
		}
		info = SessionInfo{
			User:  User{ID: r.ID, Email: r.Email, Login: r.Login, Name: r.Name, CreatedAt: r.CreatedAt},
			OrgID: p.OrgID, OrgName: r.OrgName, Role: r.Role,
		}
		return nil
	})
	return info, err
}

// Middleware puts the principal of a valid session cookie into the request
// context. A missing or invalid session is not an error: the request goes on
// without a subject and handlers decide with principal.Require.
func Middleware(s *Service) func(http.Handler) http.Handler {
	secure := s.cfg.SecureCookies
	name := sessionCookieName(secure)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c, err := r.Cookie(name); err == nil {
				if p, extended, err := s.authenticate(r.Context(), c.Value); err == nil {
					if extended {
						http.SetCookie(w, newSessionCookie(c.Value, sessionMaxAge, secure))
					}
					r = r.WithContext(principal.With(r.Context(), p))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// sessionCookieName: "__Host-" requires Secure, so development (plain http) drops the prefix.
func sessionCookieName(secure bool) string {
	if secure {
		return "__Host-werbot_session"
	}
	return "werbot_session"
}

// newSessionCookie builds the session cookie; maxAge < 0 deletes it.
func newSessionCookie(token string, maxAge int, secure bool) *http.Cookie {
	return &http.Cookie{ //nolint:gosec // Secure is false only for WERBOT_ENV=development (plain http)
		Name: sessionCookieName(secure), Value: token, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	}
}
