package jwt

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"

	profilemessage "github.com/werbot/werbot/internal/core/profile/proto/message"
)

const (
	// TokenTypeAccess marks a token that may be used to authorize API requests.
	TokenTypeAccess = "access"
	// TokenTypeRefresh marks a token that may only be exchanged for a new pair.
	TokenTypeRefresh = "refresh"

	tokenTypeClaim = "typ"
)

// UserClaims represents public and private claims for a JWT token.
type UserClaims struct {
	User profilemessage.ProfileParameters
	Type string `json:"typ"`
	jwt.RegisteredClaims
}

// TokenType returns the token style claim. An empty string means the claim is
// missing or malformed; callers must treat such tokens as invalid.
func TokenType(claims jwt.MapClaims) string {
	if claims == nil {
		return ""
	}
	tokenType, _ := claims[tokenTypeClaim].(string)
	return tokenType
}

// Parse parses a JWT token string and returns the claims if valid.
func Parse(token string) (jwt.MapClaims, error) {
	t, err := jwt.Parse(token, verifyToken)
	if err != nil {
		return nil, err
	}

	if claims, ok := t.Claims.(jwt.MapClaims); ok && t.Valid {
		return claims, nil
	}

	return nil, errors.New("token expired")
}
