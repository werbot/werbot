// Package clientip resolves the client IP address behind trusted proxies.
package clientip

import (
	"context"
	"net/http"
	"net/netip"
	"slices"
	"strings"
)

type ctxKey struct{}

// Middleware stores the client IP in the request context. When RemoteAddr is a
// trusted proxy, X-Forwarded-For is walked right to left to the first valid
// untrusted address; an invalid entry stops the walk at the last trusted hop.
func Middleware(trusted []netip.Prefix) func(http.Handler) http.Handler {
	isTrusted := func(a netip.Addr) bool {
		return slices.ContainsFunc(trusted, func(p netip.Prefix) bool { return p.Contains(a) })
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ap, err := netip.ParseAddrPort(r.RemoteAddr)
			ip := ap.Addr().Unmap()
			if err == nil && isTrusted(ip) {
				hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
				for _, h := range slices.Backward(hops) {
					a, err := netip.ParseAddr(strings.TrimSpace(h))
					if err != nil {
						break
					}
					ip = a.Unmap()
					if !isTrusted(ip) {
						break
					}
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, ip)))
		})
	}
}

// From returns the client IP stored by Middleware, or the zero Addr without it.
func From(ctx context.Context) netip.Addr {
	ip, _ := ctx.Value(ctxKey{}).(netip.Addr)
	return ip
}
