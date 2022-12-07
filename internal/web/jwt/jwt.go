package jwt

import (
	"github.com/golang-jwt/jwt/v4"

	pb "github.com/werbot/werbot/internal/grpc/proto/user"
)

// UserClaims  represents public and private claims for a JWT token.
type UserClaims struct {
	User pb.UserParameters
	jwt.RegisteredClaims
}
