package middleware

import (
	"crypto/rsa"
	"fmt"

	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/redis"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// AuthMiddleware handles authentication middleware.
type AuthMiddleware struct {
	redis     *redis.Connect
	publicKey *rsa.PublicKey
	log       logger.Logger
}

// Auth initializes the AuthMiddleware with necessary dependencies.
func Auth(redis *redis.Connect) *AuthMiddleware {
	log := logger.New()

	publicKey, err := jwt.PublicKey()
	if err != nil {
		log.Fatal(err).Send()
	}

	return &AuthMiddleware{
		log:       log,
		redis:     redis,
		publicKey: publicKey,
	}
}

// Execute is protected protect routes
func (m AuthMiddleware) Execute() fiber.Handler {
	return jwtware.New(jwtware.Config{
		// ContextKey:     "profile",
		SuccessHandler: m.authSuccess,
		ErrorHandler:   authError,
		SigningKey: jwtware.SigningKey{
			JWTAlg: jwtware.RS256,
			Key:    m.publicKey,
		},
		// SigningKey:     m.publicKey,
		// SigningMethod:  "RS256",
	})
}

// authError handles authentication errors.
func authError(c *fiber.Ctx, _ error) error {
	return webutil.FromGRPC(c, status.Error(codes.Unauthenticated, "unauthorized"))
}

// authSuccess handles successful authentication.
func (m *AuthMiddleware) authSuccess(c *fiber.Ctx) error {
	if session.TokenType(c) != jwt.TokenTypeAccess {
		return webutil.FromGRPC(c, status.Error(codes.Unauthenticated, "invalid token type"))
	}

	sessionData := session.AuthProfile(c)
	key := fmt.Sprintf("refresh_token:%s", sessionData.SessionID())

	exists, err := m.redis.Client.Exists(m.redis.Ctx, key).Result()
	if err != nil || exists == 0 {
		return webutil.FromGRPC(c, status.Error(codes.Unauthenticated, "token has been revoked"))
	}

	return c.Next()
}
