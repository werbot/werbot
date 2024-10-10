package jwt

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"

	accountpb "github.com/werbot/werbot/internal/core/account/proto/account"
)

// UserClaims represents public and private claims for a JWT token.
type UserClaims struct {
	User accountpb.UserParameters
	jwt.RegisteredClaims
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
