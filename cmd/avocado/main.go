package main

import (
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/armon/go-proxyproto"
	"github.com/gliderlabs/ssh"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/broker"
	"github.com/werbot/werbot/internal/grpc"
	rdb "github.com/werbot/werbot/internal/storage/redis"
	"github.com/werbot/werbot/pkg/logger"
)

var (
	app = App{}
)

// App is ...
type App struct {
	redis                 rdb.Handler
	grpc                  *grpc.ClientService
	defaultChannelHandler ssh.ChannelHandler
	log                   logger.Logger
	broker                broker.Handler
}

func main() {
	godotenv.Load(".env", "/etc/werbot/.env")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.log = logger.New()

	app.redis = rdb.NewClient(ctx, redis.NewClient(&redis.Options{
		Addr:     internal.GetString("REDIS_ADDR", "localhost:6379"),
		Password: internal.GetString("REDIS_PASSWORD", "redisPassword"),
	}))

	app.broker = broker.New(ctx, app.redis.Client())

	app.defaultChannelHandler = func(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {}

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

	app.grpc = grpc.NewClient(
		internal.GetString("GRPCSERVER_HOST", "localhost:50051"),
		internal.GetString("GRPCSERVER_TOKEN", "token"),
		internal.GetString("GRPCSERVER_NAMEOVERRIDE", "werbot.com"),
		grpc_certificate,
		grpc_private,
	)

	go app.broker.WriteStatus()

	// create TCP listening socket
	ln, err := net.Listen("tcp", internal.GetString("SSHSERVER_BIND_ADDRESS", ":3022"))
	if err != nil {
		app.log.Fatal(err).Msg("Start avocado server")
	}
	proxyListener := &proxyproto.Listener{Listener: ln}

	// configure server
	srv := &ssh.Server{
		Addr:    internal.GetString("SSHSERVER_BIND_ADDRESS", ":3022"),
		Version: fmt.Sprintf("[werbot] avocado-%s", internal.Version()),
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"default": channelHandler,
		},
	}

	app.defaultChannelHandler = func(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
		switch newChan.ChannelType() {
		case "session":
			go ssh.DefaultSessionHandler(srv, conn, newChan, ctx)
		case "direct-tcpip":
			go ssh.DirectTCPIPHandler(srv, conn, newChan, ctx)
		default:
			if err := newChan.Reject(gossh.UnknownChannelType, "unsupported channel type"); err != nil {
				app.log.Error(err).Msg("Failed to reject chan")
			}
		}
	}

	if internal.GetInt("SSHSERVER_IDLE_TIMEOUT", 300) != 0 {
		srv.IdleTimeout = time.Duration(internal.GetInt("SSHSERVER_IDLE_TIMEOUT", 300)) * time.Second
		srv.MaxTimeout = math.MaxInt64
	}

	for _, opt := range []ssh.Option{
		ssh.PublicKeyAuth(publicKeyAuthHandler()),
		ssh.PasswordAuth(passwordAuthHandler()),
		privateKey(),
	} {
		if err := srv.SetOption(opt); err != nil {
			app.log.Error(err).Msg("Error SetOption")
		}
	}

	log.Info().Str("serverAddress", internal.GetString("SSHSERVER_BIND_ADDRESS", ":3022")).Dur("idleTimout", time.Duration(internal.GetInt("SSHSERVER_IDLE_TIMEOUT", 300))*time.Second).Msg("SSH Server accepting connections")
	if err := srv.Serve(proxyListener); err != nil {
		app.log.Fatal(err).Msg("Create server")
	}
}
