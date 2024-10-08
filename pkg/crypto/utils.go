package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// CheckAESKey validates the length of an AES key.
func CheckAESKey(aesKey string) bool {
	switch len(aesKey) {
	case 0, 16, 24, 32:
	default:
		return false
	}
	return true
}

// AESKeyGen is
func AESKeyGen(length int) (string, error) {
	switch length {
	case 0, 16, 24, 32:
		bytes := make([]byte, length)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}
		return hex.EncodeToString(bytes), nil
	default:
		return "", fmt.Errorf("invalid key length: %d", length)
	}
}

// TextEncrypt encrypts the given text using the provided AES key.
func TextEncrypt(cryptoText, aesKey string) string {
	if cryptoText == "" || aesKey == "" {
		return cryptoText
	}
	return encrypt(cryptoText, aesKey)
}

// TextDecrypt decrypts the given text using the provided AES key.
func TextDecrypt(cryptoText, aesKey string) string {
	if cryptoText == "" || aesKey == "" {
		return cryptoText
	}
	return decrypt(cryptoText, aesKey)
}
