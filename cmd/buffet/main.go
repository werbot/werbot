package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
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

	config.Load("../../configs/.env")

	var err error
	pgDSN := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=require",
		config.GetString("POSTGRES_USER", "werbot"),
		config.GetString("POSTGRES_PASSWORD", "postgresPassword"),
		config.GetString("POSTGRES_HOST", "localhost:5432"),
		config.GetString("POSTGRES_DB", "werbot"),
	)
	db, err := postgres.ConnectDB(&postgres.PgSQLConfig{
		DSN:             pgDSN,
		MaxConn:         config.GetInt("PSQLSERVER_MAX_CONN", 50),
		MaxIdleConn:     config.GetInt("PSQLSERVER_MAX_IDLEC_CONN", 10),
		MaxLifetimeConn: config.GetInt("PSQLSERVER_MAX_LIFETIME_CONN", 300),
	})
	if err != nil {
		log.Error().Err(err).Msg("Database connection problem")
	}

	cache := cache_lib.NewRedisClient(ctx, &redis.Options{
		Addr:     config.GetString("REDIS_ADDR", "localhost:6379"),
		Password: config.GetString("REDIS_PASSWORD", "redisPassword"),
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
	lis, err := net.Listen("tcp", config.GetString("GRPCSERVER_HOST", "0.0.0.0:50051"))
	if err != nil {
		log.Fatal().Msgf("failed to listen: %v", err)
	}
	log.Info().Msgf("server listening at %v", lis.Addr())

	if err := s.GRPC.Serve(lis); err != nil {
		log.Fatal().Msgf("failed to serve: %v", err)
	}
}
