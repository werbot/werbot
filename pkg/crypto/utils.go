package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// CheckAESKey validates that the key is hex encoded and decodes to an AES key
// of 16, 24 or 32 bytes.
func CheckAESKey(aesKey string) bool {
	key, err := hex.DecodeString(aesKey)
	if err != nil {
		return false
	}

	switch len(key) {
	case 16, 24, 32:
		return true
	default:
		return false
	}
}

// AESKeyGen generates a random hex encoded AES key of the given byte length.
func AESKeyGen(length int) (string, error) {
	switch length {
	case 16, 24, 32:
		bytes := make([]byte, length)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}
		return hex.EncodeToString(bytes), nil
	default:
		return "", fmt.Errorf("%w: requested key length %d, allowed lengths are 16, 24 and 32 bytes", ErrInvalidKey, length)
	}
}

// TextEncrypt encrypts the given text using the provided AES key.
func TextEncrypt(cryptoText, aesKey string) (string, error) {
	if cryptoText == "" {
		return "", nil
	}
	return encrypt(cryptoText, aesKey)
}

// TextDecryptWithFallback decrypts with the current key and, if that fails,
// retries with an optional legacy key. usedLegacy reports which key worked so
// callers can warn that the stored secret should be re-encrypted.
func TextDecryptWithFallback(cryptoText, aesKey, legacyKey string) (plain string, usedLegacy bool, err error) {
	plain, err = TextDecrypt(cryptoText, aesKey)
	if err == nil {
		return plain, false, nil
	}
	if legacyKey == "" || legacyKey == aesKey || !CheckAESKey(legacyKey) {
		return "", false, err
	}

	plain, legacyErr := TextDecrypt(cryptoText, legacyKey)
	if legacyErr != nil {
		return "", false, err
	}
	return plain, true, nil
}

// TextDecrypt decrypts the given text using the provided AES key.
func TextDecrypt(cryptoText, aesKey string) (string, error) {
	if cryptoText == "" {
		return "", fmt.Errorf("%w: ciphertext is empty", ErrInvalidCiphertext)
	}
	return decrypt(cryptoText, aesKey)
}
