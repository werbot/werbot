package oidclogin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSafeNext shares the cases of web/src/lib/next.test.ts.
func TestSafeNext(t *testing.T) {
	for _, s := range []string{"/cli/authorize?id=1", "/", "/a", "/a?b=1", "/oauth/authorize?x=1"} {
		assert.Equal(t, s, safeNext(s), s)
	}
	for _, s := range []string{"//evil.com", "/\\evil.com", "https://evil.com", "evil.com", "/\tevil", "/a\nb", "/a\x7f", ""} {
		assert.Equal(t, "/", safeNext(s), "%q", s)
	}
}
