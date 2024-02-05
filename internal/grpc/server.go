package grpc

import (
	"crypto/tls"

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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// NewServer is ...
func NewServer(dbConn *postgres.Connect, redisConn *redis.Connect, cert tls.Certificate) *grpc.Server {
	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewServerTLSFromCert(&cert)),
		grpc.UnaryInterceptor(ensureValidToken),
	)

	accountpb.RegisterAccountHandlersServer(grpcServer, &account.Handler{DB: dbConn})
	auditpb.RegisterAuditHandlersServer(grpcServer, &audit.Handler{DB: dbConn})
	firewallpb.RegisterFirewallHandlersServer(grpcServer, &firewall.Handler{DB: dbConn})
	serverpb.RegisterServerHandlersServer(grpcServer, &server.Handler{DB: dbConn, Redis: redisConn})
	projectpb.RegisterProjectHandlersServer(grpcServer, &project.Handler{DB: dbConn})
	memberpb.RegisterMemberHandlersServer(grpcServer, &member.Handler{DB: dbConn})
	userpb.RegisterUserHandlersServer(grpcServer, &user.Handler{DB: dbConn})
	licensepb.RegisterLicenseHandlersServer(grpcServer, &license.Handler{})
	infopb.RegisterInfoHandlersServer(grpcServer, &info.Handler{DB: dbConn})
	keypb.RegisterKeyHandlersServer(grpcServer, &key.Handler{DB: dbConn, Redis: redisConn})
	utilitypb.RegisterUtilityHandlersServer(grpcServer, &utility.Handler{DB: dbConn})
	eventpb.RegisterEventHandlersServer(grpcServer, &event.Handler{DB: dbConn})

	return grpcServer
}
