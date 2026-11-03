package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

var (
	// ErrInvalidKey is returned when an AES key is not a valid hex encoded 16, 24 or 32 byte value.
	ErrInvalidKey = errors.New("invalid AES key")
	// ErrInvalidCiphertext is returned when encrypted data cannot be decoded or is too short.
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	// ErrDecrypt is returned when AES-GCM authentication or decryption fails.
	ErrDecrypt = errors.New("failed to decrypt")
)

func decodeKey(keyString string) ([]byte, error) {
	key, err := hex.DecodeString(keyString)
	if err != nil {
		return nil, fmt.Errorf("%w: key is not valid hex", ErrInvalidKey)
	}
	switch len(key) {
	case 16, 24, 32:
		return key, nil
	default:
		return nil, fmt.Errorf("%w: decoded key must be 16, 24 or 32 bytes, got %d", ErrInvalidKey, len(key))
	}
}

// Encrypt encrypts stringToEncrypt with an AES-GCM key and returns hex encoded ciphertext.
func encrypt(stringToEncrypt, keyString string) (string, error) {
	key, err := decodeKey(keyString)
	if err != nil {
		return "", err
	}
	plaintext := []byte(stringToEncrypt)

	// Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}

	// Create a new GCM - https://en.wikipedia.org/wiki/Galois/Counter_Mode
	// https://golang.org/pkg/crypto/cipher/#NewGCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}

	// Create a nonce. Nonce should be from GCM
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the data using aesGCM.Seal
	// Since we don't want to save the nonce somewhere else in this case, we add it as a prefix to the encrypted data. The first nonce argument in Seal is the prefix.
	cipherText := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return fmt.Sprintf("%x", cipherText), nil
}

// Decrypt decrypts hex encoded ciphertext produced by encrypt.
func decrypt(encryptedString, keyString string) (string, error) {
	key, err := decodeKey(keyString)
	if err != nil {
		return "", err
	}

	enc, err := hex.DecodeString(encryptedString)
	if err != nil {
		return "", fmt.Errorf("%w: data is not valid hex", ErrInvalidCiphertext)
	}

	// Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}

	// Create a new GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}

	// Get the nonce size
	nonceSize := aesGCM.NonceSize()
	if len(enc) < nonceSize {
		return "", fmt.Errorf("%w: data is shorter than the %d byte nonce", ErrInvalidCiphertext, nonceSize)
	}

	// Extract the nonce from the encrypted data
	nonce, cipherText := enc[:nonceSize], enc[nonceSize:]

	// Decrypt the data
	plaintext, err := aesGCM.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrDecrypt, err)
	}

	return string(plaintext), nil
}
