package jwt

import (
	"crypto"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"

	"github.com/golang-jwt/jwt/v4"
)

// JWK is a JSON Web Key, described in detail in RFC 7517.
type JWK struct {
	KeyType   string `json:"kty"`
	Algorithm string `json:"alg"`
	N         string `json:"n"`
	E         string `json:"e"`
}

// MarshalJWK will marshal a supported public key into JWK format.
func MarshalJWK(bytes []byte) (JWK, error) {
	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(bytes)
	if err != nil {
		return JWK{}, err
	}

	return JWK{
		KeyType:   "RSA",
		Algorithm: "RS256",
		N:         base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
		E:         base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()),
	}, nil
}

// UnmarshalJWK will unmarshal JWK into a crypto.PublicKey that can be used
// to validate signatures.
func UnmarshalJWK(jwk JWK) (crypto.PublicKey, error) {
	if jwk.KeyType != "RSA" {
		return nil, fmt.Errorf("unsupported key type %v", jwk.KeyType)
	}
	if jwk.Algorithm != "RS256" {
		return nil, fmt.Errorf("unsupported algorithm %v", jwk.Algorithm)
	}

	n, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, err
	}
	e, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, err
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(n),
		E: int(new(big.Int).SetBytes(e).Uint64()),
	}, nil
}
