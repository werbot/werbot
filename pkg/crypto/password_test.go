package crypto

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_HashPassword is ...
func Test_HashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{
			name:     "test0_01",
			password: "user6@werbot.net",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashPassword, _ := HashPassword(tt.password, 13)
			status := CheckPasswordHash(tt.password, hashPassword)
			assert.Truef(t, status, "Hash for password [%s] was incorrect", tt.password)
		})
	}
}

// Test_NewPassword verifies that NewPassword produces strings of the requested
// length, restricted to the expected alphabet, and that consecutive
// invocations yield different values (a smoke-test that the generator is no
// longer seeded deterministically).
func Test_NewPassword(t *testing.T) {
	const simpleAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const hardAlphabet = "*&^%$#@()<?!>" + simpleAlphabet

	tests := []struct {
		name     string
		length   int
		hard     bool
		alphabet string
	}{
		{name: "simple_8", length: 8, hard: false, alphabet: simpleAlphabet},
		{name: "simple_37", length: 37, hard: false, alphabet: simpleAlphabet},
		{name: "hard_22", length: 22, hard: true, alphabet: hardAlphabet},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := make(map[string]struct{}, 16)
			for range 16 {
				got := NewPassword(tt.length, tt.hard)
				assert.Len(t, got, tt.length, "length mismatch")
				for _, r := range got {
					assert.Truef(t, strings.ContainsRune(tt.alphabet, r),
						"character %q is outside the expected alphabet", r)
				}
				seen[got] = struct{}{}
			}
			assert.Greaterf(t, len(seen), 1,
				"expected NewPassword to produce different values across calls, got %d unique", len(seen))
		})
	}
}

// Test_NewPassword_NonPositive ensures that non-positive lengths return an
// empty string rather than panicking.
func Test_NewPassword_NonPositive(t *testing.T) {
	assert.Equal(t, "", NewPassword(0, false))
	assert.Equal(t, "", NewPassword(-5, true))
}
