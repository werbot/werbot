package main

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/armon/go-proxyproto"
	"github.com/gliderlabs/ssh"

	"github.com/werbot/werbot/internal/config"
	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/logger"
	"github.com/werbot/werbot/internal/storage/nats"
	"github.com/werbot/werbot/internal/version"
)

var (
	component = "avocado"
	log       = logger.NewLogger(component)
	app       = App{}
)

// App is ...
type App struct {
	nats                  *nats.Service
	grpc                  *grpc.ClientService
	defaultChannelHandler ssh.ChannelHandler
}

func main() {
	rand.Seed(time.Now().UnixNano())

	config.Load(fmt.Sprintf("../../.vscode/config/.env.%s", component))

	app.nats = nats.NewNATS(config.GetString("NATSSERVER_DSN", "nats://localhost:4222"))
	app.defaultChannelHandler = func(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {}

	app.grpc = grpc.NewClient(
		config.GetString("GRPCSERVER_DSN", "localhost:50051"),
		config.GetString("GRPCSERVER_TOKEN", "token"),
		config.GetString("GRPCSERVER_NAMEOVERRIDE", "werbot.com"),
		config.GetByteFromFile("GRPCSERVER_PUBLIC_KEY", "./grpc_public.key"),
		config.GetByteFromFile("GRPCSERVER_PRIVATE_KEY", "./grpc_private.key"),
	)

	// app.nats.WriteStatus()

	// create TCP listening socket
	ln, err := net.Listen("tcp", config.GetString("SSHSERVER_BIND_ADDRESS", ":3022"))
	if err != nil {
		log.Fatal().Err(err).Msg("Start avocado server")
	}
	proxyListener := &proxyproto.Listener{Listener: ln}

	// configure server
	srv := &ssh.Server{
		Addr:    config.GetString("SSHSERVER_BIND_ADDRESS", ":3022"),
		Version: fmt.Sprintf("[werbot] avocado-%s", version.Version()),
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
				log.Error().Err(err).Msg("Failed to reject chan")
			}
		}
	}

	if config.GetInt("SSHSERVER_IDLE_TIMEOUT", 300) != 0 {
		srv.IdleTimeout = time.Duration(config.GetInt("SSHSERVER_IDLE_TIMEOUT", 300)) * time.Second
		srv.MaxTimeout = math.MaxInt64
	}

	for _, opt := range []ssh.Option{
		ssh.PublicKeyAuth(publicKeyAuthHandler()),
		ssh.PasswordAuth(passwordAuthHandler()),
		privateKey(),
	} {
		if err := srv.SetOption(opt); err != nil {
			log.Error().Err(err).Msg("Error SetOption")
		}
	}

	log.Info().Str("serverAddress", config.GetString("SSHSERVER_BIND_ADDRESS", ":3022")).Dur("idleTimout", time.Duration(config.GetInt("SSHSERVER_IDLE_TIMEOUT", 300))*time.Second).Msg("SSH Server accepting connections")
	if err := srv.Serve(proxyListener); err != nil {
		log.Fatal().Err(err).Msg("Create server")
	}
}
