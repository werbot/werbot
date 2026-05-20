package crypto

import (
	"crypto/rand"

	"golang.org/x/crypto/bcrypt"
)

// NewPassword generates a cryptographically secure random string of length n.
// When hard is true, the alphabet additionally includes a set of special
// characters. The function relies on crypto/rand so the produced value is
// suitable for security-sensitive use cases such as passwords, secret tokens
// and API keys.
//
// To avoid modulo bias when mapping random bytes onto the alphabet the
// implementation uses rejection sampling: bytes that fall outside the largest
// multiple of the alphabet length within [0, 256) are discarded and re-drawn.
func NewPassword(n int, hard bool) string {
	if n <= 0 {
		return ""
	}

	const simpleAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	alphabet := simpleAlphabet
	if hard {
		alphabet = "*&^%$#@()<?!>" + simpleAlphabet
	}

	alphabetLen := len(alphabet)
	maxByte := 256 - (256 % alphabetLen)

	result := make([]byte, n)
	// Slightly over-allocate to reduce the number of rand.Read calls when
	// rejection sampling discards some bytes.
	buf := make([]byte, n+n/4+8)

	i := 0
	for i < n {
		if _, err := rand.Read(buf); err != nil {
			// crypto/rand.Read is documented to never fail on supported
			// platforms; a failure here means the OS entropy source is
			// broken and silently degrading to a weaker generator would be
			// dangerous.
			panic("crypto/rand: " + err.Error())
		}
		for _, b := range buf {
			if int(b) >= maxByte {
				continue
			}
			result[i] = alphabet[int(b)%alphabetLen]
			i++
			if i >= n {
				break
			}
		}
	}

	return string(result)
}

// HashPassword accepts a string password argument and returns a bcrypt hashed
// password as a string and an error.
func HashPassword(password string, cost int) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(bytes), err
}

// CheckPasswordHash compares the given password and hash for a match.
// It returns true if they match; otherwise, it returns false.
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
