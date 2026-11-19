package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBrowserCommand(t *testing.T) {
	for goos, want := range map[string][]string{
		"linux":   {"xdg-open", "u"},
		"darwin":  {"open", "u"},
		"windows": {"rundll32", "url.dll,FileProtocolHandler", "u"},
	} {
		assert.Equal(t, want, browserCommand(goos, "u"), goos)
	}
}
