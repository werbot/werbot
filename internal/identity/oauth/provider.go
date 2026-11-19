package oauth

import (
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// Provider is the authorization server: discovery, JWKS and the /oauth/ endpoints.
type Provider struct {
	handler http.Handler
	users   Users
	logger  *slog.Logger
}

// New builds the provider. Every endpoint lives under /oauth/; op's login
// callback is /oauth/authorize/callback. An http issuer is allowed only as
// such (op.WithAllowInsecure iff the scheme is http).
func New(pool *pgxpool.Pool, key *SigningKey, users Users, cfg Config, logger *slog.Logger) (*Provider, error) {
	c := &op.Config{
		CryptoKey:             key.crypto,
		CodeMethodS256:        true,
		GrantTypeRefreshToken: true,
		DeviceAuthorization: op.DeviceAuthorizationConfig{
			Lifetime: 10 * time.Minute, PollInterval: 5 * time.Second,
			UserFormURL: cfg.WebURL + "/device", UserCode: op.UserCodeBase20,
		},
	}
	opts := []op.Option{
		op.WithCustomAuthEndpoint(op.NewEndpoint("oauth/authorize")),
		op.WithCustomTokenEndpoint(op.NewEndpoint("oauth/token")),
		op.WithCustomDeviceAuthorizationEndpoint(op.NewEndpoint("oauth/device_authorization")),
		op.WithCustomKeysEndpoint(op.NewEndpoint("oauth/keys")),
		op.WithCustomRevocationEndpoint(op.NewEndpoint("oauth/revoke")),
		op.WithCustomUserinfoEndpoint(op.NewEndpoint("oauth/userinfo")),
		op.WithCustomIntrospectionEndpoint(op.NewEndpoint("oauth/introspect")),
		op.WithCustomEndSessionEndpoint(op.NewEndpoint("oauth/end_session")),
		// No CORS: the CLI does not need it and the UI reaches /oauth/ same-origin.
		op.WithCORSOptions(nil),
	}
	if u, err := url.Parse(cfg.Issuer); err == nil && u.Scheme == "http" {
		opts = append(opts, op.WithAllowInsecure())
	}
	o, err := op.NewProvider(c, &storage{pool: pool, key: key, client: cliClient{webURL: cfg.WebURL}}, op.StaticIssuer(cfg.Issuer), opts...)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/", o)
	// RFC 8414 metadata: the same document as openid-configuration.
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		r = r.Clone(r.Context())
		r.URL.Path = oidc.DiscoveryEndpoint
		o.ServeHTTP(w, r)
	})
	return &Provider{handler: mux, users: users, logger: logger}, nil
}

// Handler serves /.well-known/openid-configuration,
// /.well-known/oauth-authorization-server and /oauth/*.
func (p *Provider) Handler() http.Handler { return p.handler }
