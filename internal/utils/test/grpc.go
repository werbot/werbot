package test

import (
	"context"
	"log"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"github.com/werbot/werbot/internal/grpc/account"
	accountpb "github.com/werbot/werbot/internal/grpc/account/proto"
	"github.com/werbot/werbot/internal/grpc/audit"
	auditpb "github.com/werbot/werbot/internal/grpc/audit/proto"
	"github.com/werbot/werbot/internal/grpc/event"
	eventpb "github.com/werbot/werbot/internal/grpc/event/proto"
	"github.com/werbot/werbot/internal/grpc/firewall"
	firewallpb "github.com/werbot/werbot/internal/grpc/firewall/proto"
	"github.com/werbot/werbot/internal/grpc/info"
	infopb "github.com/werbot/werbot/internal/grpc/info/proto"
	"github.com/werbot/werbot/internal/grpc/key"
	keypb "github.com/werbot/werbot/internal/grpc/key/proto"
	"github.com/werbot/werbot/internal/grpc/license"
	licensepb "github.com/werbot/werbot/internal/grpc/license/proto"
	"github.com/werbot/werbot/internal/grpc/member"
	memberpb "github.com/werbot/werbot/internal/grpc/member/proto"
	"github.com/werbot/werbot/internal/grpc/project"
	projectpb "github.com/werbot/werbot/internal/grpc/project/proto"
	"github.com/werbot/werbot/internal/grpc/server"
	serverpb "github.com/werbot/werbot/internal/grpc/server/proto"
	"github.com/werbot/werbot/internal/grpc/user"
	userpb "github.com/werbot/werbot/internal/grpc/user/proto"
	"github.com/werbot/werbot/internal/grpc/utility"
	utilitypb "github.com/werbot/werbot/internal/grpc/utility/proto"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/internal/storage/redis"
)

type GRPCService struct {
	*grpc.ClientConn

	test  *testing.T
	db    *postgres.Connect
	redis redis.Handler
}

// GRPC is ...
func GRPC(ctx context.Context, t *testing.T, db *postgres.Connect, redis redis.Handler) (*GRPCService, error) {
	service := &GRPCService{
		test:  t,
		db:    db,
		redis: redis,
	}
	conn, err := grpc.DialContext(ctx, "", grpc.WithInsecure(), grpc.WithContextDialer(service.serverGRPC()))
	if err != nil {
		return nil, err
	}
	service.ClientConn = conn

	return service, nil
}

func (s *GRPCService) serverGRPC() func(context.Context, string) (net.Conn, error) {
	listener := bufconn.Listen(1024 * 1024)
	newServer := grpc.NewServer()

	accountpb.RegisterAccountHandlersServer(newServer, &account.Handler{DB: s.db})
	auditpb.RegisterAuditHandlersServer(newServer, &audit.Handler{DB: s.db})
	firewallpb.RegisterFirewallHandlersServer(newServer, &firewall.Handler{DB: s.db})
	infopb.RegisterInfoHandlersServer(newServer, &info.Handler{DB: s.db})
	keypb.RegisterKeyHandlersServer(newServer, &key.Handler{DB: s.db, Redis: s.redis})
	licensepb.RegisterLicenseHandlersServer(newServer, &license.Handler{})
	memberpb.RegisterMemberHandlersServer(newServer, &member.Handler{DB: s.db})
	projectpb.RegisterProjectHandlersServer(newServer, &project.Handler{DB: s.db})
	serverpb.RegisterServerHandlersServer(newServer, &server.Handler{DB: s.db, Redis: s.redis})
	userpb.RegisterUserHandlersServer(newServer, &user.Handler{DB: s.db})
	utilitypb.RegisterUtilityHandlersServer(newServer, &utility.Handler{DB: s.db})
	eventpb.RegisterEventHandlersServer(newServer, &event.Handler{DB: s.db})

	go func() {
		if err := newServer.Serve(listener); err != nil {
			log.Fatalf("Server GRPC exited with error: %v", err)
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
