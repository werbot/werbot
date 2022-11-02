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

	"github.com/werbot/werbot/internal/cache"
	"github.com/werbot/werbot/internal/config"
	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/logger"
	"github.com/werbot/werbot/internal/message"
	"github.com/werbot/werbot/internal/version"

	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/module/auth"
	"github.com/werbot/werbot/internal/web/module/customer"
	"github.com/werbot/werbot/internal/web/module/info"
	"github.com/werbot/werbot/internal/web/module/key"
	"github.com/werbot/werbot/internal/web/module/license"
	"github.com/werbot/werbot/internal/web/module/member"
	"github.com/werbot/werbot/internal/web/module/ping"
	"github.com/werbot/werbot/internal/web/module/project"
	"github.com/werbot/werbot/internal/web/module/server"
	"github.com/werbot/werbot/internal/web/module/user"
	"github.com/werbot/werbot/internal/web/module/utility"
)

var (
	component = "taco"
	log       = logger.NewLogger(component)
	app       *fiber.App
)

func main() {
	rand.Seed(time.Now().UnixNano())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config.Load(fmt.Sprintf("../../.vscode/config/.env.%s", component))
	appPort := config.GetString("APP_PORT", ":3000")

	grpcClient := grpc.NewClient(
		config.GetString("GRPCSERVER_DSN", "localhost:50051"),
		config.GetString("GRPCSERVER_TOKEN", "token"),
		config.GetString("GRPCSERVER_NAMEOVERRIDE", "werbot.com"),
		config.GetByteFromFile("GRPCSERVER_PUBLIC_KEY", "./grpc_public.key"),
		config.GetByteFromFile("GRPCSERVER_PRIVATE_KEY", "./grpc_private.key"),
	)

	cache := cache.NewRedisClient(ctx, &redis.Options{
		Addr:     config.GetString("REDIS_ADDR", "localhost:6379"),
		Password: config.GetString("REDIS_PASSWORD", ""),
	})

	ln, err := net.Listen("tcp", appPort)
	if err != nil {
		log.Fatal().Err(err).Msg(fmt.Sprintf("Error %s server", component))
	}
	proxyListener := &proxyproto.Listener{Listener: ln}

	app = fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ServerHeader:          fmt.Sprintf("[werbot] %s-%s", component, version.Version()),
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

	ping.NewHandler(app).Routes()
	auth.NewHandler(app, grpcClient, cache).Routes()

	customer.NewHandler(app, grpcClient, cache).Routes()
	key.NewHandler(app, grpcClient, cache).Routes()
	member.NewHandler(app, grpcClient, cache).Routes()
	project.NewHandler(app, grpcClient, cache).Routes()
	server.NewHandler(app, grpcClient, cache).Routes()
	info.NewHandler(app, grpcClient, cache).Routes()
	user.NewHandler(app, grpcClient, cache).Routes()
	utility.NewHandler(app, grpcClient).Routes()

	// license server
	license.NewHandler(app, grpcClient, cache, config.GetString("KEY_PUBLIC", "")).Routes()

	// dynamic handlers
	handler(app, grpcClient, cache)

	// notFoundRoute func for describe 404 Error route.
	app.Use(func(c *fiber.Ctx) error {
		return httputil.StatusNotFound(c, message.ErrNotFound, nil)
	})

	log.Info().Str("serverAddress", appPort).Msg(fmt.Sprintf("Start %s server", component))
	if err := app.Listener(proxyListener); err != nil {
		log.Fatal().Err(err).Msg("Create server")
	}
}
