package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"

	"github.com/joho/godotenv"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/version"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/storage/redis"
	"github.com/werbot/werbot/pkg/worker/asynq"
)

var log = logger.New()

func main() {
	// Load config from environment variables
	godotenv.Load(".env", "/etc/werbot/.env")

	// Create a context to control the lifetime of operations performed by this service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to PostgreSQL database
	pgDSN := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=require",
		internal.GetString("POSTGRES_USER", "werbot"),
		internal.GetString("POSTGRES_PASSWORD", "postgresPassword"),
		internal.GetString("POSTGRES_HOST", "localhost:5432"),
		internal.GetString("POSTGRES_DB", "werbot"),
	)
	db, err := postgres.New(ctx, &postgres.PgSQLConfig{
		DSN:             pgDSN,
		MaxConn:         internal.GetInt("PSQLSERVER_MAX_CONN", 50),
		MaxIdleConn:     internal.GetInt("PSQLSERVER_MAX_IDLEC_CONN", 10),
		MaxLifetimeConn: internal.GetInt("PSQLSERVER_MAX_LIFETIME_CONN", 300),
	})
	if err != nil {
		log.Fatal(err).Msg("Failed to connect to database")
	}

	// Connect to Redis
	redis := redis.New(ctx, &redis.Config{
		Addr:     internal.GetString("REDIS_ADDR", "localhost:6379"),
		Password: internal.GetString("REDIS_PASSWORD", "redisPassword"),
	})

	// Init asynq client
	asynq, err := asynq.NewClient(fmt.Sprintf("redis://:%s@%s/%s",
		internal.GetString("REDIS_PASSWORD", "redisPassword"),
		internal.GetString("REDIS_ADDR", "localhost:6379"),
		"1",
	))
	if err != nil {
		fmt.Print(err)
	}

	// Load TLS configuration from files at startup
	cert, err := tls.LoadX509KeyPair(
		internal.GetString("GRPCSERVER_CERTIFICATE", "./grpc_certificate.key"),
		internal.GetString("GRPCSERVER_PRIVATE_KEY", "./grpc_private.key"),
	)
	if err != nil {
		log.Fatal().Msg("Failed to parse GRPC keys pair")
		os.Exit(1)
	}

	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		log.Fatal(err).Msg("Failed to parse certificate")
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(cert.Leaf)

	// Initialize the GRPC server with dependencies and launch it
	serverAddr := internal.GetString("GRPCSERVER_HOST", "0.0.0.0:50051")
	log.Info().Str("serverAddress", serverAddr).Str("version", version.Version()).Msg("Starting buffet server")

	s := grpc.NewServer(db, redis, asynq, cert)

	lis, err := net.Listen("tcp", serverAddr)
	if err != nil {
		log.Fatal(err).Msg("Failed to listen")
	}
	if err := s.Serve(lis); err != nil {
		log.Fatal(err).Msg("Failed to serve")
	}
}
