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
	"github.com/werbot/werbot/internal/grpc/audit"
	"github.com/werbot/werbot/internal/grpc/firewall"
	"github.com/werbot/werbot/internal/grpc/info"
	"github.com/werbot/werbot/internal/grpc/key"
	"github.com/werbot/werbot/internal/grpc/license"
	"github.com/werbot/werbot/internal/grpc/logging"
	"github.com/werbot/werbot/internal/grpc/member"
	"github.com/werbot/werbot/internal/grpc/project"
	"github.com/werbot/werbot/internal/grpc/server"
	"github.com/werbot/werbot/internal/grpc/user"
	"github.com/werbot/werbot/internal/grpc/utility"
	"github.com/werbot/werbot/internal/storage/cache"
	"github.com/werbot/werbot/internal/storage/postgres"

	accountpb "github.com/werbot/werbot/api/proto/account"
	auditpb "github.com/werbot/werbot/api/proto/audit"
	firewallpb "github.com/werbot/werbot/api/proto/firewall"
	infopb "github.com/werbot/werbot/api/proto/info"
	keypb "github.com/werbot/werbot/api/proto/key"
	licensepb "github.com/werbot/werbot/api/proto/license"
	loggingpb "github.com/werbot/werbot/api/proto/logging"
	memberpb "github.com/werbot/werbot/api/proto/member"
	projectpb "github.com/werbot/werbot/api/proto/project"
	serverpb "github.com/werbot/werbot/api/proto/server"
	userpb "github.com/werbot/werbot/api/proto/user"
	utilitypb "github.com/werbot/werbot/api/proto/utility"
)

var service Service

// Service is ...
type Service struct {
	DB    *postgres.Connect
	Cache cache.Cache
}

// CreateGRPC is ...
func CreateGRPC(ctx context.Context, t *testing.T, s *Service) *grpc.ClientConn {
	service.DB = s.DB
	service.Cache = s.Cache

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
	keypb.RegisterKeyHandlersServer(tServer, &key.Handler{DB: service.DB, Cache: service.Cache})
	licensepb.RegisterLicenseHandlersServer(tServer, &license.Handler{})
	memberpb.RegisterMemberHandlersServer(tServer, &member.Handler{DB: service.DB})
	projectpb.RegisterProjectHandlersServer(tServer, &project.Handler{DB: service.DB})
	serverpb.RegisterServerHandlersServer(tServer, &server.Handler{DB: service.DB, Cache: service.Cache})
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
