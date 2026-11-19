package app

import (
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/config"
)

// mountOAuth builds the CLI authorization server and mounts /oauth/,
// /.well-known/, OAuthConsentService and CLISessionService on mux, with or
// without the embedded UI. Tokens carry iss = WERBOT_PUBLIC_URL; consent and
// device pages live at WERBOT_WEB_URL. The provider verifies Bearer tokens
// for identity.Middleware.
func mountOAuth(mux *http.ServeMux, cfg config.Config, pool *pgxpool.Pool, key *oauth.SigningKey, ids *identity.Service, logger *slog.Logger, opts ...connect.HandlerOption) (*oauth.Provider, error) {
	p, err := oauth.New(pool, key, ids, oauth.Config{Issuer: cfg.PublicURL, WebURL: cfg.WebURL}, logger)
	if err != nil {
		return nil, err
	}
	mux.Handle("/oauth/", p.Handler())
	mux.Handle("/.well-known/", p.Handler())
	mux.Handle(werbotv1connect.NewOAuthConsentServiceHandler(oauth.NewConsentHandler(p), opts...))
	mux.Handle(werbotv1connect.NewCLISessionServiceHandler(identity.NewHandler(ids), opts...))
	return p, nil
}
