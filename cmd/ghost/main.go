package main

import (
	"fmt"

	"github.com/joho/godotenv"

	"github.com/werbot/werbot/internal"
	grpc "github.com/werbot/werbot/internal/core"
	"github.com/werbot/werbot/internal/version"
	"github.com/werbot/werbot/internal/worker"
	"github.com/werbot/werbot/pkg/logger"
)

var log logger.Logger

func main() {
	godotenv.Load(".env", "/etc/werbot/.env")

	log = logger.New()
	log.Info().Str("version", version.Version()).Msg("Start ghost server")

	grpcClient, err := grpc.NewClient()
	if err != nil {
		log.Fatal(err).Send()
	}
	defer grpcClient.Close()

	redisURI := fmt.Sprintf("redis://:%s@%s/%s",
		internal.GetString("REDIS_PASSWORD", "redisPassword"),
		internal.GetString("REDIS_ADDR", "localhost:6379"),
		"1",
	)

	worker.New(redisURI, grpcClient)

	// hack
	var input string
	fmt.Scanf("%v", &input)
}
