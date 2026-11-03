package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/werbot/werbot/internal"
	grpc "github.com/werbot/werbot/internal/core"
	"github.com/werbot/werbot/internal/version"
	"github.com/werbot/werbot/internal/worker"
	"github.com/werbot/werbot/pkg/logger"
)

var log logger.Logger

func main() {
	if err := run(); err != nil {
		log.Fatal(err).Send()
	}
}

func run() error {
	godotenv.Load(".env", "/etc/werbot/.env")

	log = logger.New()
	log.Info().Str("version", version.Version()).Msg("Start ghost server")

	grpcClient, err := grpc.NewClient()
	if err != nil {
		return err
	}
	defer grpcClient.Close()

	redisURI := fmt.Sprintf("redis://:%s@%s/%s",
		internal.GetString("REDIS_PASSWORD", "redisPassword"),
		internal.GetString("REDIS_ADDR", "localhost:6379"),
		"1",
	)

	workerServer, err := worker.New(redisURI, grpcClient)
	if err != nil {
		return err
	}
	defer workerServer.Shutdown()

	// The asynq server starts in the background, so the process has to block
	// until it is asked to stop. Without this, a detached container would exit
	// immediately because stdin is EOF.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Info().Msg("Shutting down ghost server")

	return nil
}
