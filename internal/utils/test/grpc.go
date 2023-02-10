package test

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"github.com/werbot/werbot/internal/grpc/account"
	accountpb "github.com/werbot/werbot/internal/grpc/account/proto"
	"github.com/werbot/werbot/internal/grpc/audit"
	auditpb "github.com/werbot/werbot/internal/grpc/audit/proto"
	"github.com/werbot/werbot/internal/grpc/firewall"
	firewallpb "github.com/werbot/werbot/internal/grpc/firewall/proto"
	"github.com/werbot/werbot/internal/grpc/info"
	infopb "github.com/werbot/werbot/internal/grpc/info/proto"
	"github.com/werbot/werbot/internal/grpc/key"
	keypb "github.com/werbot/werbot/internal/grpc/key/proto"
	"github.com/werbot/werbot/internal/grpc/license"
	licensepb "github.com/werbot/werbot/internal/grpc/license/proto"
	"github.com/werbot/werbot/internal/grpc/logging"
	loggingpb "github.com/werbot/werbot/internal/grpc/logging/proto"
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

var service Service

// Service is ...
type Service struct {
	DB    *postgres.Connect
	Redis redis.Handler
}

// CreateGRPC is ...
func CreateGRPC(ctx context.Context, t *testing.T, s *Service) *grpc.ClientConn {
	service.DB = s.DB
	service.Redis = s.Redis

	conn, err := grpc.DialContext(ctx, "", grpc.WithInsecure(), grpc.WithContextDialer(serverGRPC(t)))
	require.NoError(t, err)

	return conn
}

func serverGRPC(t *testing.T) func(context.Context, string) (net.Conn, error) {
	listener := bufconn.Listen(1024 * 1024)
	tServer := grpc.NewServer()

	accountpb.RegisterAccountHandlersServer(tServer, &account.Handler{DB: service.DB})
	auditpb.RegisterAuditHandlersServer(tServer, &audit.Handler{DB: service.DB})
	firewallpb.RegisterFirewallHandlersServer(tServer, &firewall.Handler{DB: service.DB})
	infopb.RegisterInfoHandlersServer(tServer, &info.Handler{DB: service.DB})
	keypb.RegisterKeyHandlersServer(tServer, &key.Handler{DB: service.DB, Redis: service.Redis})
	licensepb.RegisterLicenseHandlersServer(tServer, &license.Handler{})
	memberpb.RegisterMemberHandlersServer(tServer, &member.Handler{DB: service.DB})
	projectpb.RegisterProjectHandlersServer(tServer, &project.Handler{DB: service.DB})
	serverpb.RegisterServerHandlersServer(tServer, &server.Handler{DB: service.DB, Redis: service.Redis})
	userpb.RegisterUserHandlersServer(tServer, &user.Handler{DB: service.DB})
	utilitypb.RegisterUtilityHandlersServer(tServer, &utility.Handler{DB: service.DB})
	loggingpb.RegisterLoggingHandlersServer(tServer, &logging.Handler{DB: service.DB})

	go func() {
		if err := tServer.Serve(listener); err != nil {
			log.Fatalf("Server GRPC exited with error: %v", err)
		}
	}()

	return func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
}
