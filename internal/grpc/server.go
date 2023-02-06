package grpc

import (
	"crypto/tls"

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
	"github.com/werbot/werbot/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var service Service

// Service is ...
type Service struct {
	db    *postgres.Connect
	cache cache.Cache
	log   logger.Logger
	token string
}

// ServerService is ...
type ServerService struct {
	GRPC *grpc.Server
}

// NewServer is ...
func NewServer(token string, dbConn *postgres.Connect, cacheConn cache.Cache, cert tls.Certificate) *ServerService {
	log := logger.New("internal/grpc")
	service = Service{
		log:   log,
		token: token,
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewServerTLSFromCert(&cert)),
		grpc.UnaryInterceptor(ensureValidToken),
	)

	accountpb.RegisterAccountHandlersServer(grpcServer, &account.Handler{DB: dbConn})
	auditpb.RegisterAuditHandlersServer(grpcServer, &audit.Handler{DB: dbConn})
	firewallpb.RegisterFirewallHandlersServer(grpcServer, &firewall.Handler{DB: dbConn})
	serverpb.RegisterServerHandlersServer(grpcServer, &server.Handler{DB: dbConn, Cache: cacheConn})
	projectpb.RegisterProjectHandlersServer(grpcServer, &project.Handler{DB: dbConn})
	memberpb.RegisterMemberHandlersServer(grpcServer, &member.Handler{DB: dbConn})
	userpb.RegisterUserHandlersServer(grpcServer, &user.Handler{DB: dbConn})
	licensepb.RegisterLicenseHandlersServer(grpcServer, &license.Handler{})
	infopb.RegisterInfoHandlersServer(grpcServer, &info.Handler{DB: dbConn})
	keypb.RegisterKeyHandlersServer(grpcServer, &key.Handler{DB: dbConn, Cache: cacheConn})
	utilitypb.RegisterUtilityHandlersServer(grpcServer, &utility.Handler{DB: dbConn})
	loggingpb.RegisterLoggingHandlersServer(grpcServer, &logging.Handler{DB: dbConn})

	return &ServerService{
		GRPC: grpcServer,
	}
}
