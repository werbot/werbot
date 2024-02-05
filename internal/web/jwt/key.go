package jwt

import (
	"crypto/rsa"
	"fmt"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"github.com/werbot/werbot/internal"
)

// Define three global variables to store public, private keys and sync value. Initialize publicKey and privateKey to nil.
var (
	publicKey  *rsa.PublicKey
	privateKey *rsa.PrivateKey
	once       sync.Once
)

// The loadKeys function reads the public and private key files, parses them into appropriate data types and stores them in their respective variables.
// It returns an error if there's any issue with reading file, parsing keys or storing them.
func loadKeys() error {
	// Read content of JWT_PUBLIC_KEY file into a byte array.
	pubBytes, err := internal.GetByteFromFile("JWT_PUBLIC_KEY", "./jwt_public.key")
	if err != nil {
		return fmt.Errorf("failed to read public key file: %w", err)
	}

	// Read content of JWT_PRIVATE_KEY file into a byte array.
	privBytes, err := internal.GetByteFromFile("JWT_PRIVATE_KEY", "./jwt_private.key")
	if err != nil {
		return fmt.Errorf("failed to read private key file: %w", err)
	}

	// Parse RSA Public Key from PEM encoded bytes
	pubKey, err := jwt.ParseRSAPublicKeyFromPEM(pubBytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	// Parse RSA Private Key from PEM encoded bytes
	privKey, err := jwt.ParseRSAPrivateKeyFromPEM(privBytes)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Store public and private keys
	publicKey = pubKey
	privateKey = privKey

	return nil
}

// PublicKey is a function that returns a pointer to an rsa.PublicKey and an error.
func PublicKey() (*rsa.PublicKey, error) {
	once.Do(func() { _ = loadKeys() })
	if publicKey == nil {
		return nil, fmt.Errorf("public key not loaded")
	}
	return publicKey, nil
}

// PrivateKey is a function that returns a pointer to an rsa.PrivateKey and an error.
func PrivateKey() (*rsa.PrivateKey, error) {
	once.Do(func() { _ = loadKeys() })
	if privateKey == nil {
		return nil, fmt.Errorf("private key not loaded")
	}
	return privateKey, nil
}
