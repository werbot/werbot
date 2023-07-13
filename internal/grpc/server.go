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
	"github.com/werbot/werbot/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var service Service

// Service is ...
type Service struct {
	// db    *postgres.Connect
	// redis redis.Handler
	log   logger.Logger
	token string
}

// ServerService is ...
type ServerService struct {
	GRPC *grpc.Server
}

// NewServer is ...
func NewServer(token string, dbConn *postgres.Connect, redisConn redis.Handler, cert tls.Certificate) *ServerService {
	log := logger.New()
	service = Service{
		log:   log,
		token: token,
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewServerTLSFromCert(&cert)),
		grpc.UnaryInterceptor(ensureValidToken),
	)

	accountpb.RegisterAccountHandlersServer(grpcServer, &account.Handler{DB: dbConn, Log: log})
	auditpb.RegisterAuditHandlersServer(grpcServer, &audit.Handler{DB: dbConn, Log: log})
	firewallpb.RegisterFirewallHandlersServer(grpcServer, &firewall.Handler{DB: dbConn, Log: log})
	serverpb.RegisterServerHandlersServer(grpcServer, &server.Handler{DB: dbConn, Redis: redisConn, Log: log})
	projectpb.RegisterProjectHandlersServer(grpcServer, &project.Handler{DB: dbConn, Log: log})
	memberpb.RegisterMemberHandlersServer(grpcServer, &member.Handler{DB: dbConn, Log: log})
	userpb.RegisterUserHandlersServer(grpcServer, &user.Handler{DB: dbConn, Log: log})
	licensepb.RegisterLicenseHandlersServer(grpcServer, &license.Handler{Log: log})
	infopb.RegisterInfoHandlersServer(grpcServer, &info.Handler{DB: dbConn, Log: log})
	keypb.RegisterKeyHandlersServer(grpcServer, &key.Handler{DB: dbConn, Redis: redisConn, Log: log})
	utilitypb.RegisterUtilityHandlersServer(grpcServer, &utility.Handler{DB: dbConn, Log: log})
	eventpb.RegisterEventHandlersServer(grpcServer, &event.Handler{DB: dbConn, Log: log})

	return &ServerService{
		GRPC: grpcServer,
	}
}
