package test

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	igrpc "github.com/werbot/werbot/internal/grpc"
)

// GRPCService holds the gRPC client connection and dependencies.
type GRPCService struct {
	*grpc.ClientConn
	db    *PostgresService
	redis *RedisService
	test  *testing.T
}

// ServerGRPC initializes a new GRPCService with a gRPC client connection.
func ServerGRPC(ctx context.Context, t *testing.T, db *PostgresService, redis *RedisService) (*GRPCService, error) {
	service := &GRPCService{
		test:  t,
		db:    db,
		redis: redis,
	}

	var err error
	service.ClientConn, err = grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(service.serverGRPC()))
	if err != nil {
		return nil, err
	}

	return service, nil
}

// serverGRPC sets up the in-memory gRPC server and registers all handlers.
func (s *GRPCService) serverGRPC() func(context.Context, string) (net.Conn, error) {
	listener := bufconn.Listen(1024 * 1024)
	newServer := grpc.NewServer()
	newServer = igrpc.ServerHandlers(newServer, s.db.conn, s.redis.conn, s.redis.worker)

	go func() {
		if err := newServer.Serve(listener); err != nil {
			s.test.Fatalf("Server GRPC exited with error: %v", err)
		}
	}()

	return func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
}

// Close is ...
func (s *GRPCService) Close() {
	if err := s.ClientConn.Close(); err != nil {
		s.test.Error(err)
	}
}
