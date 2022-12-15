package main

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/armon/go-proxyproto"
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/helmet/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/logger"
	"github.com/werbot/werbot/internal/storage/cache"

	"github.com/werbot/werbot/api/web/auth"
	"github.com/werbot/werbot/api/web/customer"
	"github.com/werbot/werbot/api/web/info"
	"github.com/werbot/werbot/api/web/key"
	"github.com/werbot/werbot/api/web/license"
	"github.com/werbot/werbot/api/web/member"
	"github.com/werbot/werbot/api/web/ping"
	"github.com/werbot/werbot/api/web/project"
	"github.com/werbot/werbot/api/web/server"
	"github.com/werbot/werbot/api/web/user"
	"github.com/werbot/werbot/api/web/utility"
	"github.com/werbot/werbot/api/web/wellknown"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"
)

var (
	component = "taco"
	log       = logger.New(component)
	app       *fiber.App
)

func main() {
	rand.Seed(time.Now().UnixNano())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	internal.LoadConfig("../../.env")
	appPort := internal.GetString("APP_PORT", ":8088")

	grpcClient := grpc.NewClient(
		internal.GetString("GRPCSERVER_HOST", "localhost:50051"),
		internal.GetString("GRPCSERVER_TOKEN", "token"),
		internal.GetString("GRPCSERVER_NAMEOVERRIDE", "werbot.com"),
		internal.GetByteFromFile("GRPCSERVER_PUBLIC_KEY", "./grpc_public.key"),
		internal.GetByteFromFile("GRPCSERVER_PRIVATE_KEY", "./grpc_private.key"),
	)

	cache := cache.New(ctx, &redis.Options{
		Addr:     internal.GetString("REDIS_ADDR", "localhost:6379"),
		Password: internal.GetString("REDIS_PASSWORD", "redisPassword"),
	})

	ln, err := net.Listen("tcp", appPort)
	if err != nil {
		log.Fatal().Err(err).Msg(fmt.Sprintf("Error %s server", component))
	}
	proxyListener := &proxyproto.Listener{Listener: ln}

	app = fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ServerHeader:          fmt.Sprintf("[werbot] %s-%s", component, internal.Version()),
	})

	app.Use(
		cors.New(cors.Config{
			AllowOrigins:     "*",
			AllowMethods:     "GET,POST,HEAD,OPTIONS,PUT,DELETE,PATCH",
			AllowHeaders:     "Origin, Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Requested-With",
			ExposeHeaders:    "Origin",
			AllowCredentials: true,
		}),
		helmet.New(),
		etag.New(),
	)

	ping.New(app).Routes()
	wellknown.New(app).Routes()

	authMiddleware := middleware.Auth(cache).Execute()
	auth.New(app, grpcClient, cache, authMiddleware).Routes()

	customer.New(app, grpcClient, authMiddleware).Routes()
	key.New(app, grpcClient, authMiddleware).Routes()
	member.New(app, grpcClient, authMiddleware).Routes()
	project.New(app, grpcClient, authMiddleware).Routes()
	server.New(app, grpcClient, authMiddleware).Routes()
	info.New(app, grpcClient, authMiddleware).Routes()
	user.New(app, grpcClient, authMiddleware).Routes()
	utility.New(app, grpcClient).Routes()

	// license server
	license.New(app, grpcClient, authMiddleware, internal.GetString("LICENSE_KEY_PUBLIC", "")).Routes()

	// dynamic handlers
	handler(app, grpcClient, authMiddleware)

	// notFoundRoute func for describe 404 Error route.
	app.Use(func(c *fiber.Ctx) error {
		return httputil.StatusNotFound(c, internal.ErrNotFound, nil)
	})

	log.Info().Str("serverAddress", appPort).Msg(fmt.Sprintf("Start %s server", component))
	if err := app.Listener(proxyListener); err != nil {
		log.Fatal().Err(err).Msg("Create server")
	}
}
