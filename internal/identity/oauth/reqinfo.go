package oauth

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"unicode"

	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/werbot/werbot/internal/platform/clientip"
)

const (
	maxBody     = 64 << 10 // bytes of an /oauth/ form
	maxHostname = 255      // bytes of werbot_hostname
)

// reqInfo is what the storage needs from the HTTP request op is serving:
// the CLI's reported hostname and its address, and the approved device code
// a poll found (op hands the token step only a *op.DeviceAuthorizationState).
// A token exchange also carries werbot_client and werbot_reason, and its
// grant from the validate step to the token step.
type reqInfo struct {
	hostname       string
	ip             netip.Addr
	device         *deviceGrant
	exchange       bool
	client, reason string
	grant          *agentGrant
}

type reqInfoKey struct{}

// requestInfo returns the reqInfo of the request in ctx (zero outside one).
func requestInfo(ctx context.Context) *reqInfo {
	if ri, ok := ctx.Value(reqInfoKey{}).(*reqInfo); ok {
		return ri
	}
	return &reqInfo{}
}

// ipPtr is the client address for a nullable column: nil when unknown.
func (ri *reqInfo) ipPtr() *netip.Addr {
	if !ri.ip.IsValid() {
		return nil
	}
	return &ri.ip
}

// withReqInfo runs before op: it caps the body, refuses an authorization
// request whose client or redirect_uri is not exactly werbot-cli on a
// loopback /callback (400, never a redirect to it) and records reqInfo.
func withReqInfo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		if err := r.ParseForm(); err != nil {
			status := http.StatusBadRequest
			if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
				status = http.StatusRequestEntityTooLarge
			}
			http.Error(w, "invalid request", status)
			return
		}
		if r.URL.Path == "/oauth/authorize" && !validAuthorizeTarget(r.Form) {
			http.Error(w, "invalid client_id or redirect_uri", http.StatusBadRequest)
			return
		}
		ri := &reqInfo{hostname: cleanHostname(r.Form.Get("werbot_hostname")), ip: clientip.From(r.Context())}
		if r.URL.Path == "/oauth/token" && r.Form.Get("grant_type") == string(oidc.GrantTypeTokenExchange) {
			exchangeInfo(r, ri)
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), reqInfoKey{}, ri)))
	})
}

// validAuthorizeTarget: one client_id, werbot-cli, and one redirect_uri
// http://127.0.0.1 or http://[::1], any port, path /callback, nothing else.
// op takes the last of repeated values and accepts localhost and https, so
// both are checked here, before op can redirect anywhere.
func validAuthorizeTarget(form url.Values) bool {
	if len(form["client_id"]) != 1 || form.Get("client_id") != ClientCLI || len(form["redirect_uri"]) != 1 {
		return false
	}
	u, err := url.Parse(form.Get("redirect_uri"))
	if err != nil || u.Scheme != "http" || u.User != nil || u.Opaque != "" ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.EscapedPath() != "/callback" {
		return false
	}
	h := u.Hostname()
	return h == "127.0.0.1" || h == "::1"
}

// exchangeInfo records a token exchange: werbot_client and werbot_reason
// without control characters (checked by the storage). op reads the exchange
// client from Basic auth only, so one form client_id werbot-cli becomes Basic
// werbot-cli with an empty secret when the request has no Authorization.
func exchangeInfo(r *http.Request, ri *reqInfo) {
	ri.exchange = true
	ri.client, ri.reason = dropControl(r.Form.Get("werbot_client")), dropControl(r.Form.Get("werbot_reason"))
	if len(r.Form["client_id"]) == 1 && r.Form.Get("client_id") == ClientCLI && r.Header.Get("Authorization") == "" {
		r.SetBasicAuth(ClientCLI, "")
	}
}

// cleanHostname drops control characters and keeps at most maxHostname
// bytes of valid UTF-8.
func cleanHostname(s string) string {
	s = dropControl(s)
	if len(s) > maxHostname {
		s = s[:maxHostname]
	}
	return strings.ToValidUTF8(s, "")
}

// dropControl removes control characters.
func dropControl(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}
