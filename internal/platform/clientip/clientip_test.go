package clientip_test

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/werbot/werbot/internal/platform/clientip"
)

func TestClientIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	cases := []struct{ remote, xff, want string }{
		{"203.0.113.9:5000", "1.2.3.4", "203.0.113.9"},
		{"10.0.0.2:5000", "1.2.3.4, 10.0.0.3", "1.2.3.4"},
		{"10.0.0.2:5000", "garbage", "10.0.0.2"},
		{"[::ffff:10.0.0.2]:5000", "", "10.0.0.2"},
	}
	for _, c := range cases {
		var got netip.Addr
		h := clientip.Middleware(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			got = clientip.From(r.Context())
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = c.remote
		if c.xff != "" {
			req.Header.Set("X-Forwarded-For", c.xff)
		}
		h.ServeHTTP(httptest.NewRecorder(), req)
		assert.Equal(t, netip.MustParseAddr(c.want), got, c.remote+" "+c.xff)
	}
	assert.False(t, clientip.From(httptest.NewRequest(http.MethodGet, "/", nil).Context()).IsValid())
}
