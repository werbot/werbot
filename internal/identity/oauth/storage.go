package oauth

import (
	"context"
	"errors"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// errNoFlow answers the steps of a flow this server does not serve.
var errNoFlow = errors.New("oauth: flow not supported")

// storage is the op.Storage (and op.DeviceAuthorizationStorage,
// op.TokenExchangeStorage) of the provider: keys, the werbot-cli client,
// health, the agent runs of the token exchange (agents nil: not served) and
// the IdP check of a refresh (idpCheck nil: none).
type storage struct {
	pool     *pgxpool.Pool
	key      *SigningKey
	client   cliClient
	agents   Agents
	idpCheck func(ctx context.Context, orgID, userID uuid.UUID) error
}

// The device grant is advertised only while storage implements this.
var _ op.DeviceAuthorizationStorage = (*storage)(nil)

// Health implements op.Storage.
func (s *storage) Health(ctx context.Context) error { return s.pool.Ping(ctx) }

// SigningKey implements op.AuthStorage.
func (s *storage) SigningKey(context.Context) (op.SigningKey, error) { return s.key, nil }

// SignatureAlgorithms implements op.AuthStorage: ES256 only.
func (*storage) SignatureAlgorithms(context.Context) ([]jose.SignatureAlgorithm, error) {
	return []jose.SignatureAlgorithm{jose.ES256}, nil
}

// KeySet implements op.AuthStorage: the public signing key (JWKS).
func (s *storage) KeySet(context.Context) ([]op.Key, error) { return []op.Key{publicKey{s.key}}, nil }

// GetClientByClientID implements op.OPStorage: werbot-cli only.
func (s *storage) GetClientByClientID(_ context.Context, clientID string) (op.Client, error) {
	if clientID != ClientCLI {
		return nil, oidc.ErrInvalidClient().WithDescription("unknown client")
	}
	return s.client, nil
}

// AuthorizeClientIDSecret implements op.OPStorage: werbot-cli has no secret.
// op authenticates the token exchange client only this way, so the exchange
// alone accepts werbot-cli with an empty secret; everything else is refused.
func (*storage) AuthorizeClientIDSecret(ctx context.Context, clientID, secret string) error {
	if requestInfo(ctx).exchange && clientID == ClientCLI && secret == "" {
		return nil
	}
	return oidc.ErrInvalidClient().WithDescription("client secrets are not supported")
}

// TerminateSession implements op.AuthStorage: the CLI ends its session with
// /oauth/revoke; end_session has nothing to end.
func (*storage) TerminateSession(context.Context, string, string) error { return nil }

// SetUserinfoFromScopes implements op.OPStorage: op copies the userinfo into
// the ID token, sub included, so it carries only sub.
func (*storage) SetUserinfoFromScopes(_ context.Context, info *oidc.UserInfo, userID, _ string, _ []string) error {
	info.Subject = userID
	return nil
}

// SetUserinfoFromToken implements op.OPStorage: userinfo carries only sub.
func (*storage) SetUserinfoFromToken(_ context.Context, info *oidc.UserInfo, _, subject, _ string) error {
	info.Subject = subject
	return nil
}

// SetIntrospectionFromToken implements op.OPStorage: introspection needs an
// authenticated client and werbot-cli cannot authenticate.
func (*storage) SetIntrospectionFromToken(context.Context, *oidc.IntrospectionResponse, string, string, string) error {
	return errNoFlow
}

// GetPrivateClaimsFromScopes implements op.OPStorage: no claims from scopes.
func (*storage) GetPrivateClaimsFromScopes(context.Context, string, string, []string) (map[string]any, error) {
	return nil, nil
}

// GetKeyByIDAndClientID implements op.OPStorage: no JWT profile grant.
func (*storage) GetKeyByIDAndClientID(context.Context, string, string) (*jose.JSONWebKey, error) {
	return nil, errNoFlow
}

// ValidateJWTProfileScopes implements op.OPStorage: no JWT profile grant.
func (*storage) ValidateJWTProfileScopes(context.Context, string, []string) ([]string, error) {
	return nil, errNoFlow
}
