package jwt

import (
	"errors"

	"github.com/golang-jwt/jwt/v4"

	authpb "github.com/werbot/werbot/api/proto/auth"
)

// UserClaims  represents public and private claims for a JWT token.
type UserClaims struct {
	User authpb.UserParameters
	jwt.RegisteredClaims
}

// Parse is ...
func Parse(token string) (*jwt.MapClaims, error) {
	t, err := jwt.Parse(token, VerifyToken)
	if err != nil {
		return nil, errors.New("Token parsing error")
	}

	claims, ok := t.Claims.(jwt.MapClaims)
	if ok && t.Valid {
		return &claims, nil
	}

	return nil, errors.New("Token expired")
}

// GetClaimSub is ...
func GetClaimSub(claim jwt.MapClaims) string {
	return claim["sub"].(string)
}
