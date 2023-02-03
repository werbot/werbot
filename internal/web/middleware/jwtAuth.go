package middleware

import (
	"crypto/rsa"
	"fmt"

	"github.com/gofiber/fiber/v2"
	jwtware "github.com/gofiber/jwt/v3"

	"github.com/werbot/werbot/internal/storage/cache"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/webutil"
)

const (
	msgTokenHasBeenRevoked = "token has been revoked"
)

// AuthMiddleware is ...
type AuthMiddleware struct {
	cache     cache.Cache
	publicKey *rsa.PublicKey
	log       logger.Logger
}

// Auth is ...
func Auth(cache cache.Cache) *AuthMiddleware {
	log := logger.New("middleware/auth")

	publicKey, err := jwt.PublicKey()
	if err != nil {
		log.Fatal(err).Send()
	}

	return &AuthMiddleware{
		log:       log,
		cache:     cache,
		publicKey: publicKey,
	}
}

// Execute is protected protect routes
func (m AuthMiddleware) Execute() fiber.Handler {
	return jwtware.New(jwtware.Config{
		SuccessHandler: m.authSuccess,
		ErrorHandler:   authError,
		SigningKey:     m.publicKey,
		SigningMethod:  "RS256",
	})
}

func authError(c *fiber.Ctx, e error) error {
	return webutil.StatusUnauthorized(c, "Unauthorized", nil)
}

func (m AuthMiddleware) authSuccess(c *fiber.Ctx) error {
	userInfo := AuthUser(c)

	key := fmt.Sprintf("ref_token::%s", userInfo.UserSub())
	if _, err := m.cache.Get(key).Result(); err != nil {
		return webutil.StatusUnauthorized(c, msgTokenHasBeenRevoked, nil)
	}

	return c.Next()
}
