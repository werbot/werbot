package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/joho/godotenv"

	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/api/agent"
	"github.com/werbot/werbot/api/auth"
	"github.com/werbot/werbot/api/event"
	"github.com/werbot/werbot/api/key"
	"github.com/werbot/werbot/api/license"
	"github.com/werbot/werbot/api/member"
	"github.com/werbot/werbot/api/ping"
	"github.com/werbot/werbot/api/profile"
	"github.com/werbot/werbot/api/project"
	"github.com/werbot/werbot/api/scheme"
	"github.com/werbot/werbot/api/system"
	"github.com/werbot/werbot/api/token"
	"github.com/werbot/werbot/api/websocket"
	"github.com/werbot/werbot/api/wellknown"
	"github.com/werbot/werbot/internal"
	grpc "github.com/werbot/werbot/internal/core"
	"github.com/werbot/werbot/internal/version"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/redis"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

var (
	log = logger.New()
	app *fiber.App
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err).Send()
	}
}

func run() error {
	godotenv.Load(".env", "/etc/werbot/.env")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	appPort := internal.GetString("APP_PORT", ":8088")

	grpcClient, err := grpc.NewClient()
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	redis := redis.New(ctx, &redis.Config{
		Addr:     internal.GetString("REDIS_ADDR", "localhost:6379"),
		Password: internal.GetString("REDIS_PASSWORD", "redisPassword"),
	})

	app = fiber.New(fiber.Config{
		// HAProxy sets X-Real-IP to the real client address. Only trust the
		// header when the request comes from a trusted proxy, otherwise a
		// client could spoof its IP and bypass the auth rate limiter.
		ProxyHeader:             "X-Real-IP",
		EnableIPValidation:      true,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          internal.GetSliceString("TRUSTED_PROXIES", "127.0.0.1,::1,10.16.0.0/16"),
		DisableStartupMessage:   true,
		ServerHeader:            fmt.Sprintf("[werbot] %s-%s", "taco", version.Version()),
	})

	app.Use(
		cors.New(cors.Config{
			AllowOrigins:  "*",
			AllowMethods:  "GET,POST,HEAD,OPTIONS,PUT,DELETE,PATCH",
			AllowHeaders:  "Origin, Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Requested-With",
			ExposeHeaders: "Origin",
			// AllowCredentials: true,
		}),
		helmet.New(),
		etag.New(),
		// Throttle credential endpoints to slow password and token brute force.
		// Only sign-in and password-reset are limited: refreshing a token and
		// logging out are normal high-frequency operations behind a shared NAT.
		limiter.New(limiter.Config{
			Max:        20,
			Expiration: time.Minute,
			Next: func(c *fiber.Ctx) bool {
				path := c.Path()
				return path != "/auth/signin" && !strings.HasPrefix(path, "/auth/password_reset")
			},
		}),
	)

	authMiddleware := middleware.Auth(redis).Execute()
	webHandler := &api.Handler{
		App:     app,
		Grpc:    grpcClient,
		Redis:   redis,
		Auth:    authMiddleware,
		EnvMode: internal.GetString("ENV_MODE", "prod"),
	}

	ping.New(webHandler).Routes()
	wellknown.New(webHandler).Routes()

	agent.New(webHandler).Routes()
	auth.New(webHandler).Routes()
	key.New(webHandler).Routes()
	member.New(webHandler).Routes()
	project.New(webHandler).Routes()
	scheme.New(webHandler).Routes()
	profile.New(webHandler).Routes()
	system.New(webHandler).Routes()
	event.New(webHandler).Routes()
	token.New(webHandler).Routes()

	websocket.New(webHandler).Routes()

	// license server
	license.New(webHandler, internal.GetString("LICENSE_KEY_PUBLIC", "")).Routes()

	// dynamic handlers
	registerAddons(webHandler)

	// notFoundRoute func for describe 404 Error route.
	app.Use(func(c *fiber.Ctx) error {
		return webutil.StatusNotFound(c, nil)
	})

	log.Info().Str("serverAddress", appPort).Str("version", version.Version()).Msg("Start taco server")

	ln, err := net.Listen("tcp", appPort)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", appPort, err)
	}
	if err := app.Listener(ln); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}
