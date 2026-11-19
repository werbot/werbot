package oauth

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"unicode"

	"github.com/werbot/werbot/internal/platform/clientip"
)

const (
	maxBody     = 64 << 10 // bytes of an /oauth/ form
	maxHostname = 255      // bytes of werbot_hostname
)

// reqInfo is what the storage needs from the HTTP request op is serving:
// the CLI's reported hostname and its address, and the approved device code
// a poll found (op hands the token step only a *op.DeviceAuthorizationState).
type reqInfo struct {
	hostname string
	ip       netip.Addr
	device   *deviceGrant
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

// cleanHostname drops control characters and keeps at most maxHostname
// bytes of valid UTF-8.
func cleanHostname(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	if len(s) > maxHostname {
		s = s[:maxHostname]
	}
	return strings.ToValidUTF8(s, "")
}
