package app

import (
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/config"
)

// mountOAuth builds the CLI authorization server and mounts /oauth/,
// /.well-known/ and OAuthConsentService on mux, with or without the embedded
// UI. Tokens carry iss = WERBOT_PUBLIC_URL; consent and device pages live at
// WERBOT_WEB_URL.
func mountOAuth(mux *http.ServeMux, cfg config.Config, pool *pgxpool.Pool, key *oauth.SigningKey, users oauth.Users, logger *slog.Logger, opts ...connect.HandlerOption) error {
	p, err := oauth.New(pool, key, users, oauth.Config{Issuer: cfg.PublicURL, WebURL: cfg.WebURL}, logger)
	if err != nil {
		return err
	}
	mux.Handle("/oauth/", p.Handler())
	mux.Handle("/.well-known/", p.Handler())
	mux.Handle(werbotv1connect.NewOAuthConsentServiceHandler(oauth.NewConsentHandler(p), opts...))
	return nil
}
