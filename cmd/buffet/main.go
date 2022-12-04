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

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/logger"
	cache_lib "github.com/werbot/werbot/internal/storage/cache"
	"github.com/werbot/werbot/internal/storage/postgres"
)

var (
	log      = logger.New("buffet")
	cert     tls.Certificate // Cert is a self signed certificate
	certPool *x509.CertPool  // CertPool contains the self signed certificate
)

func main() {
	rand.Seed(time.Now().UnixNano())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	internal.LoadConfig("../../.env")

	var err error
	pgDSN := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=require",
		internal.GetString("POSTGRES_USER", "werbot"),
		internal.GetString("POSTGRES_PASSWORD", "postgresPassword"),
		internal.GetString("POSTGRES_HOST", "localhost:5432"),
		internal.GetString("POSTGRES_DB", "werbot"),
	)
	db, err := postgres.New(&postgres.PgSQLConfig{
		DSN:             pgDSN,
		MaxConn:         internal.GetInt("PSQLSERVER_MAX_CONN", 50),
		MaxIdleConn:     internal.GetInt("PSQLSERVER_MAX_IDLEC_CONN", 10),
		MaxLifetimeConn: internal.GetInt("PSQLSERVER_MAX_LIFETIME_CONN", 300),
	})
	if err != nil {
		log.Error().Err(err).Msg("Database connection problem")
	}

	cache := cache_lib.New(ctx, &redis.Options{
		Addr:     internal.GetString("REDIS_ADDR", "localhost:6379"),
		Password: internal.GetString("REDIS_PASSWORD", "redisPassword"),
	})

	cert, err = tls.X509KeyPair(
		internal.GetByteFromFile("GRPCSERVER_PUBLIC_KEY", "./grpc_public.key"),
		internal.GetByteFromFile("GRPCSERVER_PRIVATE_KEY", "./grpc_private.key"),
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

	s := grpc.NewServer(internal.GetString("GRPCSERVER_TOKEN", "token"), db, cache, cert)
	lis, err := net.Listen("tcp", internal.GetString("GRPCSERVER_HOST", "0.0.0.0:50051"))
	if err != nil {
		log.Fatal().Msgf("failed to listen: %v", err)
	}
	log.Info().Msgf("server listening at %v", lis.Addr())

	if err := s.GRPC.Serve(lis); err != nil {
		log.Fatal().Msgf("failed to serve: %v", err)
	}
}
