package jwt

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"

	accountpb "github.com/werbot/werbot/internal/grpc/account/proto"
)

// UserClaims  represents public and private claims for a JWT token.
type UserClaims struct {
	User accountpb.UserParameters
	jwt.RegisteredClaims
}

// Parse is ...
func Parse(token string) (jwt.MapClaims, error) {
	t, err := jwt.Parse(token, verifyToken)
	if err != nil {
		return nil, err
	}

	claims, ok := t.Claims.(jwt.MapClaims)
	if ok && t.Valid {
		return claims, nil
	}

	return nil, errors.New("Token expired")
}
