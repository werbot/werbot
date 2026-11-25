package identity

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignupLoginName(t *testing.T) {
	for email, want := range map[string]string{
		"Carol.Smith+x@example.com":    "carol-smith-x",
		"_-bob@example.com":            "bob",
		"a@example.com":                "user",
		"@example.com":                 "user",
		"--@example.com":               "user",
		"Ünal@example.com":             "nal",
		strings.Repeat("x", 40) + "@e": strings.Repeat("x", 28),
		"dev_ops-1@example.com":        "dev_ops-1",
	} {
		got := signupLogin(email)
		assert.Equal(t, want, got, email)
		assert.Regexp(t, loginPattern, got, email)
	}
	assert.Regexp(t, loginPattern, strings.Repeat("x", 28)+"-999", "the longest suffixed login fits users.login")
}
