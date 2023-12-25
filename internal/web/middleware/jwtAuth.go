package middleware

import (
	"crypto/rsa"
	"fmt"

	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal/storage/redis"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/webutil"
)

// AuthMiddleware is ...
type AuthMiddleware struct {
	redis     redis.Handler
	publicKey *rsa.PublicKey
	log       logger.Logger
}

// Auth is ...
func Auth(redis redis.Handler) *AuthMiddleware {
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

func authError(c *fiber.Ctx, e error) error {
	return webutil.FromGRPC(c, status.Error(codes.Unauthenticated, "Unauthorized"))
}

func (m AuthMiddleware) authSuccess(c *fiber.Ctx) error {
	userInfo := AuthUser(c)

	key := fmt.Sprintf("ref_token:%s", userInfo.UserSub())
	if _, err := m.redis.Get(key).Result(); err != nil {
		return webutil.FromGRPC(c, status.Error(codes.Unauthenticated, "Token has been revoked"))
	}

	return c.Next()
}
