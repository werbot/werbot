// Package clientauth is the CLI side of werbot login: the profile on disk,
// browser (loopback + PKCE) and device login, access-token refresh and SSH
// user-certificate renewal. It talks only to the server given by the user.
package clientauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	envServer = "WERBOT_SERVER"
	envHome   = "WERBOT_HOME"

	clientID = "werbot-cli"
	scope    = "openid offline_access"
)

// Errors the commands show as they are.
var (
	ErrNotLoggedIn   = errors.New("not logged in; run werbot login --server <url>")
	ErrLoginRequired = errors.New("session expired or revoked; run werbot login")
)

// ConfigVars lists the client variables of werbot login and the commands after it.
func ConfigVars() []string { return []string{envServer, envHome} }

// httpClient bounds every OAuth call; the context may have no deadline.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// normalizeServer keeps scheme and host of an http(s) server URL.
func normalizeServer(server string) (string, error) {
	u, err := url.Parse(server)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
		strings.Trim(u.Path, "/") != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("invalid server URL %q: want http(s)://host[:port]", server)
	}
	return u.Scheme + "://" + u.Host, nil
}

// tokenResponse is the RFC 6749 token answer.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// oauthError is an RFC 6749 error answer; it never carries a token.
type oauthError struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

func (e *oauthError) Error() string {
	if e.Description == "" {
		return "oauth: " + e.Code
	}
	return "oauth: " + e.Code + ": " + e.Description
}

// oauthCode returns the RFC 6749 error code of err, or "".
func oauthCode(err error) string {
	if oe, ok := errors.AsType[*oauthError](err); ok {
		return oe.Code
	}
	return ""
}

// postForm posts form to url and decodes a 200 answer into out (nil: ignored).
func postForm(ctx context.Context, u string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body := io.LimitReader(res.Body, 1<<20)
	if res.StatusCode != http.StatusOK {
		var oe oauthError
		if json.NewDecoder(body).Decode(&oe) == nil && oe.Code != "" {
			return &oe
		}
		return fmt.Errorf("%s: %s", u, res.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(body).Decode(out)
}

// tokenRequest calls <server>/oauth/token for the werbot CLI client.
func tokenRequest(ctx context.Context, server string, form url.Values) (tokenResponse, error) {
	form.Set("client_id", clientID)
	var tok tokenResponse
	if err := postForm(ctx, server+"/oauth/token", form, &tok); err != nil {
		return tokenResponse{}, err
	}
	if tok.AccessToken == "" {
		return tokenResponse{}, errors.New("oauth: token answer without access_token")
	}
	return tok, nil
}
