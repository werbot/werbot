package jwt

import (
	"crypto/rsa"

	"github.com/golang-jwt/jwt/v4"
	"github.com/werbot/werbot/internal"
)

// PublicKey is ...
func PublicKey() (*rsa.PublicKey, error) {
	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(internal.GetByteFromFile("JWT_PUBLIC_KEY", "./jwt_public.key"))
	if err != nil {
		return nil, err
	}
	return publicKey, nil
}

// PrivateKey is ...
func PrivateKey() (*rsa.PrivateKey, error) {
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(internal.GetByteFromFile("JWT_PRIVATE_KEY", "./jwt_private.key"))
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}
