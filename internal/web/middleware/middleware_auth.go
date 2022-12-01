package middleware

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	jwtware "github.com/gofiber/jwt/v3"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/storage/cache"
	"github.com/werbot/werbot/internal/web/httputil"
)

type authMiddleware struct {
	cache cache.Cache
}

// NewAuthMiddleware is ...
func NewAuthMiddleware(cache cache.Cache) Middleware {
	return authMiddleware{
		cache: cache,
	}
}

// Protected protect routes
func (m authMiddleware) Execute() fiber.Handler {
	return jwtware.New(jwtware.Config{
		SuccessHandler: m.authSuccess,
		ErrorHandler:   authError,
		SigningKey:     []byte(internal.GetString("ACCESS_TOKEN_SECRET", "accessTokenSecret")),
		SigningMethod:  "HS256",
	})
}

func authError(c *fiber.Ctx, e error) error {
	return httputil.StatusUnauthorized(c, "Unauthorized", nil)
}

func (m authMiddleware) authSuccess(c *fiber.Ctx) error {
	userParameter := GetUserParameters(c)
	key := fmt.Sprintf("ref_token::%s", userParameter.GetUserSub())
	if _, err := m.cache.Get(key); err != nil {
		return httputil.StatusUnauthorized(c, "Your token has been revoked", nil)
	}

	return c.Next()
}
