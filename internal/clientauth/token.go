package clientauth

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// refreshBefore is how close to expiry an access token is refreshed.
const refreshBefore = time.Minute

// setTokens stores a token answer; the caller holds mu or owns p.
func (p *Profile) setTokens(tok tokenResponse) {
	p.AccessToken = tok.AccessToken
	if tok.RefreshToken != "" {
		p.RefreshToken = tok.RefreshToken
	}
	p.AccessExpiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).UTC()
}

// Token returns a live access token, refreshing it under the profile lock
// when less than a minute is left. A refresh made by another process in the
// meantime is reused, so concurrent commands rotate the refresh token once.
func (p *Profile) Token(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if time.Until(p.AccessExpiry) > refreshBefore {
		return p.AccessToken, nil
	}
	unlock, err := lockProfile(ctx, Dir(p.home, p.Server))
	if err != nil {
		return "", err
	}
	defer unlock()
	disk, err := load(p.home, p.Server)
	if err != nil {
		return "", err
	}
	p.AccessToken, p.RefreshToken, p.AccessExpiry = disk.AccessToken, disk.RefreshToken, disk.AccessExpiry
	if time.Until(p.AccessExpiry) > refreshBefore {
		return p.AccessToken, nil
	}
	tok, err := tokenRequest(ctx, p.Server, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {p.RefreshToken}})
	if oauthCode(err) == "invalid_grant" {
		return "", ErrLoginRequired // the profile stays; a later command tries again (Ruling R9)
	}
	if err != nil {
		return "", err
	}
	p.setTokens(tok)
	return p.AccessToken, p.write()
}

// HTTPClient sends a fresh Bearer token with every request.
func (p *Profile) HTTPClient() *http.Client {
	return &http.Client{Transport: bearer{p}}
}

type bearer struct{ p *Profile }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	tok, err := b.p.Token(r.Context())
	if err != nil {
		if r.Body != nil {
			_ = r.Body.Close()
		}
		return nil, err
	}
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+tok)
	return http.DefaultTransport.RoundTrip(r)
}
