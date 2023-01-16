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

	"github.com/werbot/werbot/api/web"
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
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/storage/cache"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/webutil"
)

var (
	log = logger.New("taco")
	app *fiber.App
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
		internal.GetByteFromFile("GRPCSERVER_CERTIFICATE", "./grpc_certificate.key"),
		internal.GetByteFromFile("GRPCSERVER_PRIVATE_KEY", "./grpc_private.key"),
	)

	cache := cache.New(ctx, &redis.Options{
		Addr:     internal.GetString("REDIS_ADDR", "localhost:6379"),
		Password: internal.GetString("REDIS_PASSWORD", "redisPassword"),
	})

	ln, err := net.Listen("tcp", appPort)
	if err != nil {
		log.Fatal(err).Msg("Error server")
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
	webHandler := &web.Handler{
		App:   app,
		Grpc:  grpcClient,
		Cache: cache,
		Auth:  authMiddleware,
	}

	ping.New(webHandler).Routes()
	wellknown.New(webHandler).Routes()

	auth.New(webHandler).Routes()
	customer.New(webHandler).Routes()
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
		return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	})

	log.Info().Str("serverAddress", appPort).Msg("Start taco server")
	if err := app.Listener(proxyListener); err != nil {
		log.Fatal(err).Msg("Create server")
	}
}
