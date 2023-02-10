package test

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/helmet/v2"
	"github.com/werbot/werbot/internal/storage/redis"
	"google.golang.org/grpc"
)

// Web is ...
type Web struct {
	GRPC    *grpc.ClientConn
	Redis   redis.Handler
	App     *fiber.App
	Handler http.HandlerFunc
	t       *testing.T
}

// CreateWeb is ...
func CreateWeb(t *testing.T) *Web {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Use(
		cors.New(cors.Config{
			AllowOrigins:     "*",
			AllowMethods:     "GET,POST,HEAD,OPTIONS,PUT,DELETE,PATCH",
			AllowHeaders:     "Origin, Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization,X-Requested-With",
			ExposeHeaders:    "Origin",
			AllowCredentials: true,
		}),
		helmet.New(),
		etag.New(),
	)

	return &Web{
		App: app,
	}
}
