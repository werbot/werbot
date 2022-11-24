package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"math/rand"
	"net"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/werbot/werbot/internal/config"
	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/logger"
	cache_lib "github.com/werbot/werbot/internal/storage/cache"
	"github.com/werbot/werbot/internal/storage/postgres"
)

var (
	log      = logger.NewLogger("buffet")
	cert     tls.Certificate // Cert is a self signed certificate
	certPool *x509.CertPool  // CertPool contains the self signed certificate
)

func main() {
	rand.Seed(time.Now().UnixNano())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config.Load("../../.vscode/config/.env.buffet")

	var err error
	db, err := postgres.ConnectDB(&postgres.PgSQLConfig{
		DSN:             config.GetString("PSQLSERVER_DSN", "postgres://login:password@localhost:5432/werbot?sslmode=require"),
		MaxConn:         config.GetInt("PSQLSERVER_MAX_CONN", 50),
		MaxIdleConn:     config.GetInt("PSQLSERVER_MAX_IDLEC_ON", 10),
		MaxLifetimeConn: config.GetInt("PSQLSERVER_MAX_LIFETIME_CONN", 300),
	})
	if err != nil {
		log.Error().Err(err).Msg("Database connection problem")
	}

	cache := cache_lib.NewRedisClient(ctx, &redis.Options{
		Addr:     config.GetString("REDIS_ADDR", "localhost:6379"),
		Password: config.GetString("REDIS_PASSWORD", ""),
	})

	cert, err = tls.X509KeyPair(
		config.GetByteFromFile("GRPCSERVER_PUBLIC_KEY", "./grpc_public.key"),
		config.GetByteFromFile("GRPCSERVER_PRIVATE_KEY", "./grpc_private.key"),
	)
	if err != nil {
		log.Fatal().Msgf("Failed to parse key pair: %s", err)
	}

	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		log.Fatal().Msgf("Failed to parse certificate: %s", err)
	}

	certPool = x509.NewCertPool()
	certPool.AddCert(cert.Leaf)

	s := grpc.NewServer(config.GetString("GRPCSERVER_TOKEN", "token"), db, cache, cert)
	lis, err := net.Listen("tcp", config.GetString("GRPCSERVER_DSN", "0.0.0.0:50051"))
	if err != nil {
		log.Fatal().Msgf("failed to listen: %v", err)
	}
	log.Info().Msgf("server listening at %v", lis.Addr())

	if err := s.GRPC.Serve(lis); err != nil {
		log.Fatal().Msgf("failed to serve: %v", err)
	}
}
