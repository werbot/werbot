// Package secretbox seals small secrets with AES-256-GCM and supports key rotation.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

const keyLen = 32

// ErrUnknownKey means the ciphertext was sealed with a key the Box does not hold.
var ErrUnknownKey = errors.New("secretbox: unknown key id")

// Box seals with the current key and opens with the current or any previous key.
type Box struct {
	id   string
	aead map[string]cipher.AEAD
}

// New builds a Box. Every key must be 32 bytes; duplicates are allowed.
func New(current []byte, previous [][]byte) (*Box, error) {
	b := &Box{id: KeyID(current), aead: map[string]cipher.AEAD{}}
	for _, k := range append([][]byte{current}, previous...) {
		if len(k) != keyLen {
			return nil, fmt.Errorf("secretbox: key must be %d bytes, got %d", keyLen, len(k))
		}
		block, err := aes.NewCipher(k)
		if err != nil {
			return nil, fmt.Errorf("secretbox: %w", err)
		}
		g, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("secretbox: %w", err)
		}
		b.aead[KeyID(k)] = g
	}
	return b, nil
}

// KeyID returns hex(SHA-256(key))[:8].
func KeyID(key []byte) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:])[:8]
}

// KeyID returns the id of the current key.
func (b *Box) KeyID() string { return b.id }

// Seal encrypts plain with the current key; the result is nonce(12) || sealed.
func (b *Box) Seal(plain []byte) (ciphertext []byte, keyID string, err error) {
	g := b.aead[b.id]
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, "", fmt.Errorf("secretbox: nonce: %w", err)
	}
	return g.Seal(nonce, nonce, plain, nil), b.id, nil
}

// Open decrypts data sealed under keyID. An unknown keyID gives ErrUnknownKey.
func (b *Box) Open(data []byte, keyID string) ([]byte, error) {
	g, ok := b.aead[keyID]
	if !ok {
		return nil, ErrUnknownKey
	}
	n := g.NonceSize()
	if len(data) < n {
		return nil, errors.New("secretbox: ciphertext too short")
	}
	plain, err := g.Open(nil, data[:n], data[n:], nil)
	if err != nil {
		return nil, fmt.Errorf("secretbox: open: %w", err)
	}
	return plain, nil
}
