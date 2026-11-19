package app

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/config"
)

// mountOAuth builds the CLI authorization server and mounts /oauth/ and
// /.well-known/ on mux, with or without the embedded UI. Tokens carry
// iss = WERBOT_PUBLIC_URL; consent and device pages live at WERBOT_WEB_URL.
func mountOAuth(mux *http.ServeMux, cfg config.Config, pool *pgxpool.Pool, key *oauth.SigningKey, logger *slog.Logger) error {
	p, err := oauth.New(pool, key, nil, oauth.Config{Issuer: cfg.PublicURL, WebURL: cfg.WebURL}, logger)
	if err != nil {
		return err
	}
	mux.Handle("/oauth/", p.Handler())
	mux.Handle("/.well-known/", p.Handler())
	return nil
}
