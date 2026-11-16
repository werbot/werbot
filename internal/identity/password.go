package identity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// argon2id parameters (contract §3.1): time=3, memory=64 MiB, threads=2, 32-byte key, 16-byte salt.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16
)

var errMalformedHash = errors.New("identity: malformed password hash")

// dummyHash is verified when no user matches the email, so an unknown email
// costs the same argon2 work as a wrong password.
var dummyHash = encodeHash(make([]byte, argonSaltLen), make([]byte, argonKeyLen))

// hashPassword returns pw as an argon2id PHC string.
func hashPassword(pw string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	return encodeHash(salt, argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)), nil
}

func encodeHash(salt, key []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

// verifyPassword reports whether pw matches the PHC string encoded, using the
// parameters stored in it.
func verifyPassword(encoded, pw string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return false, errMalformedHash
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, errMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, errMalformedHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return false, errMalformedHash
	}
	got := argon2.IDKey([]byte(pw), salt, time, memory, threads, uint32(len(key))) //nolint:gosec // key length comes from our own 32-byte hashes
	return subtle.ConstantTimeCompare(got, key) == 1, nil
}

// validatePassword enforces the 12–256 character rule.
func validatePassword(pw string) error {
	if n := utf8.RuneCountInString(pw); n < 12 || n > 256 {
		return fmt.Errorf("password must be 12 to 256 characters: %w", apperr.ErrInvalidArgument)
	}
	return nil
}
