package main

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/armon/go-proxyproto"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/helmet/v2"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/api/auth"
	"github.com/werbot/werbot/api/info"
	"github.com/werbot/werbot/api/key"
	"github.com/werbot/werbot/api/license"
	"github.com/werbot/werbot/api/member"
	"github.com/werbot/werbot/api/ping"
	"github.com/werbot/werbot/api/project"
	"github.com/werbot/werbot/api/server"
	"github.com/werbot/werbot/api/user"
	"github.com/werbot/werbot/api/utility"
	"github.com/werbot/werbot/api/wellknown"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/grpc"
	rdb "github.com/werbot/werbot/internal/storage/redis"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/webutil"
)

var (
	log = logger.New()
	app *fiber.App
)

func main() {
	godotenv.Load(".env", "/etc/werbot/.env")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	appPort := internal.GetString("APP_PORT", ":8088")

	// Load TLS configuration from files at startup
	grpc_certificate, err := internal.GetByteFromFile("GRPCSERVER_CERTIFICATE", "./grpc_certificate.key")
	if err != nil {
		log.Fatal().Msg("Failed to open grpc_certificate.key")
		os.Exit(1)
	}

	grpc_private, err := internal.GetByteFromFile("GRPCSERVER_PRIVATE_KEY", "./grpc_private.key")
	if err != nil {
		log.Fatal().Msg("Failed to open grpc_private.key")
		os.Exit(1)
	}

	grpcClient := grpc.NewClient(
		internal.GetString("GRPCSERVER_HOST", "localhost:50051"),
		internal.GetString("GRPCSERVER_TOKEN", "token"),
		internal.GetString("GRPCSERVER_NAMEOVERRIDE", "werbot.com"),
		grpc_certificate,
		grpc_private,
	)

	cache := rdb.NewClient(ctx, redis.NewClient(&redis.Options{
		Addr:     internal.GetString("REDIS_ADDR", "localhost:6379"),
		Password: internal.GetString("REDIS_PASSWORD", "redisPassword"),
	}))

	ln, err := net.Listen("tcp", appPort)
	if err != nil {
		log.Fatal(err).Send()
	}
	proxyListener := &proxyproto.Listener{Listener: ln}

	app = fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ServerHeader:          fmt.Sprintf("[werbot] %s-%s", "taco", internal.Version()),
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

	authMiddleware := middleware.Auth(cache).Execute()
	webHandler := &api.Handler{
		App:   app,
		Grpc:  grpcClient,
		Redis: cache,
		Auth:  authMiddleware,
	}

	ping.New(webHandler).Routes()
	wellknown.New(webHandler).Routes()

	auth.New(webHandler).Routes()
	info.New(webHandler).Routes()
	key.New(webHandler).Routes()
	member.New(webHandler).Routes()
	project.New(webHandler).Routes()
	server.New(webHandler).Routes()
	user.New(webHandler).Routes()
	utility.New(webHandler).Routes()

	// license server
	license.New(webHandler, internal.GetString("LICENSE_KEY_PUBLIC", "")).Routes()

	// dynamic handlers
	handler(webHandler)

	// notFoundRoute func for describe 404 Error route.
	app.Use(func(c *fiber.Ctx) error {
		return webutil.Response(c, fiber.StatusNotFound, "", nil)
	})

	log.Info().Str("serverAddress", appPort).Msg("Start taco server")
	if err := app.Listener(proxyListener); err != nil {
		log.Fatal(err).Msg("Create server")
	}
}
