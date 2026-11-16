package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// newSecret returns 32 random bytes as base64url (43 characters) and the
// SHA-256 of that string, which is all the database keeps.
func newSecret() (raw string, hash []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // never returns an error
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashSecret(raw)
}

func hashSecret(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
