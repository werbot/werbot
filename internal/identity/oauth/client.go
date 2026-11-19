package oauth

import (
	"net/url"
	"time"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// cliClient is werbot-cli: public (no secret), native, loopback redirects
// (RFC 8252 §7.3: any port), authorization code, refresh and device grants.
type cliClient struct{ webURL string }

func (cliClient) GetID() string { return ClientCLI }

func (cliClient) RedirectURIs() []string {
	return []string{"http://127.0.0.1/callback", "http://[::1]/callback"}
}

func (cliClient) PostLogoutRedirectURIs() []string    { return nil }
func (cliClient) ApplicationType() op.ApplicationType { return op.ApplicationTypeNative }
func (cliClient) AuthMethod() oidc.AuthMethod         { return oidc.AuthMethodNone }
func (cliClient) ResponseTypes() []oidc.ResponseType {
	return []oidc.ResponseType{oidc.ResponseTypeCode}
}
func (cliClient) AccessTokenType() op.AccessTokenType                          { return op.AccessTokenTypeJWT }
func (cliClient) IDTokenLifetime() time.Duration                               { return AccessTokenTTL }
func (cliClient) DevMode() bool                                                { return false }
func (cliClient) IsScopeAllowed(string) bool                                   { return false }
func (cliClient) IDTokenUserinfoClaimsAssertion() bool                         { return false }
func (cliClient) ClockSkew() time.Duration                                     { return 0 }
func (cliClient) RestrictAdditionalIdTokenScopes() func([]string) []string     { return keepScopes } //nolint:revive // op.Client names it
func (cliClient) RestrictAdditionalAccessTokenScopes() func([]string) []string { return keepScopes }

func (cliClient) GrantTypes() []oidc.GrantType {
	return []oidc.GrantType{oidc.GrantTypeCode, oidc.GrantTypeRefreshToken, oidc.GrantTypeDeviceCode}
}

// LoginURL is the consent page of the web UI (outside the proxied /oauth/).
func (c cliClient) LoginURL(id string) string {
	return c.webURL + "/cli/authorize?id=" + url.QueryEscape(id)
}

func keepScopes(scopes []string) []string { return scopes }
