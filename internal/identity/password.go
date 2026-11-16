package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"runtime"
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

// argonSlots bounds concurrent argon2 runs: each allocates its memory
// parameter (64 MiB), so public Login cannot exhaust the process memory.
// ponytail: per-process cap of GOMAXPROCS runs (~GOMAXPROCS × 64 MiB); callers
// queue on it. Rate limiting of Login and RequestPasswordReset is a later stage.
var argonSlots = make(chan struct{}, runtime.GOMAXPROCS(0))

// argonKey is argon2.IDKey run in a free slot; it gives up with ctx.Err()
// when ctx ends first.
func argonKey(ctx context.Context, pw, salt []byte, time, memory uint32, threads uint8, keyLen uint32) ([]byte, error) {
	select {
	case argonSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-argonSlots }()
	return argon2.IDKey(pw, salt, time, memory, threads, keyLen), nil
}

// hashPassword returns pw as an argon2id PHC string.
func hashPassword(ctx context.Context, pw string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := argonKey(ctx, []byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	if err != nil {
		return "", err
	}
	return encodeHash(salt, key), nil
}

func encodeHash(salt, key []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

// verifyPassword reports whether pw matches the PHC string encoded, using the
// parameters stored in it.
func verifyPassword(ctx context.Context, encoded, pw string) (bool, error) {
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
	got, err := argonKey(ctx, []byte(pw), salt, time, memory, threads, uint32(len(key))) //nolint:gosec // key length comes from our own 32-byte hashes
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, key) == 1, nil
}

// loginPattern is the users.login CHECK (migrations/00003_users.sql).
var loginPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,31}$`)

// validateLogin checks login against the users.login CHECK.
func validateLogin(login string) error {
	if !loginPattern.MatchString(login) {
		return fmt.Errorf("login must match %s: %w", loginPattern, apperr.ErrInvalidArgument)
	}
	return nil
}

// validatePassword enforces the 12–256 character rule.
func validatePassword(pw string) error {
	if n := utf8.RuneCountInString(pw); n < 12 || n > 256 {
		return fmt.Errorf("password must be 12 to 256 characters: %w", apperr.ErrInvalidArgument)
	}
	return nil
}
