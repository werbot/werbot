package grpc

import (
	"crypto/tls"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	//"github.com/werbot/werbot/internal/grpc/info"
	//infopb "github.com/werbot/werbot/internal/grpc/info/proto/info"

	//"github.com/werbot/werbot/internal/grpc/utility"
	//utilitypb "github.com/werbot/werbot/internal/grpc/utility/proto/utility"

	"github.com/werbot/werbot/internal/grpc/account"
	accountpb "github.com/werbot/werbot/internal/grpc/account/proto/account"
	"github.com/werbot/werbot/internal/grpc/agent"
	agentpb "github.com/werbot/werbot/internal/grpc/agent/proto/agent"
	"github.com/werbot/werbot/internal/grpc/audit"
	auditpb "github.com/werbot/werbot/internal/grpc/audit/proto/audit"
	"github.com/werbot/werbot/internal/grpc/event"
	eventpb "github.com/werbot/werbot/internal/grpc/event/proto/event"
	"github.com/werbot/werbot/internal/grpc/firewall"
	firewallpb "github.com/werbot/werbot/internal/grpc/firewall/proto/firewall"
	"github.com/werbot/werbot/internal/grpc/invite"
	invitepb "github.com/werbot/werbot/internal/grpc/invite/proto/invite"
	"github.com/werbot/werbot/internal/grpc/key"
	keypb "github.com/werbot/werbot/internal/grpc/key/proto/key"
	"github.com/werbot/werbot/internal/grpc/license"
	licensepb "github.com/werbot/werbot/internal/grpc/license/proto/license"
	"github.com/werbot/werbot/internal/grpc/member"
	memberpb "github.com/werbot/werbot/internal/grpc/member/proto/member"
	"github.com/werbot/werbot/internal/grpc/notification"
	notificationpb "github.com/werbot/werbot/internal/grpc/notification/proto/notification"
	"github.com/werbot/werbot/internal/grpc/project"
	projectpb "github.com/werbot/werbot/internal/grpc/project/proto/project"
	"github.com/werbot/werbot/internal/grpc/scheme"
	schemepb "github.com/werbot/werbot/internal/grpc/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/grpc/system"
	systempb "github.com/werbot/werbot/internal/grpc/system/proto/system"
	"github.com/werbot/werbot/internal/grpc/user"
	userpb "github.com/werbot/werbot/internal/grpc/user/proto/user"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/storage/redis"
	"github.com/werbot/werbot/pkg/worker"
)

// NewServer is ...
func NewServer(dbConn *postgres.Connect, redisConn *redis.Connect, asynq worker.Client, cert tls.Certificate) *grpc.Server {
	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewServerTLSFromCert(&cert)),
		grpc.UnaryInterceptor(ensureValidToken),
	)

	return ServerHandlers(grpcServer, dbConn, redisConn, asynq)
}

func ServerHandlers(grpcServer *grpc.Server, dbConn *postgres.Connect, redisConn *redis.Connect, asynq worker.Client) *grpc.Server {
	accountpb.RegisterAccountHandlersServer(grpcServer, &account.Handler{DB: dbConn, Worker: asynq})
	agentpb.RegisterAgentHandlersServer(grpcServer, &agent.Handler{DB: dbConn, Redis: redisConn})
	auditpb.RegisterAuditHandlersServer(grpcServer, &audit.Handler{DB: dbConn})
	firewallpb.RegisterFirewallHandlersServer(grpcServer, &firewall.Handler{DB: dbConn})
	invitepb.RegisterInviteHandlersServer(grpcServer, &invite.Handler{DB: dbConn})
	schemepb.RegisterSchemeHandlersServer(grpcServer, &scheme.Handler{DB: dbConn, Redis: redisConn})
	projectpb.RegisterProjectHandlersServer(grpcServer, &project.Handler{DB: dbConn})
	memberpb.RegisterMemberHandlersServer(grpcServer, &member.Handler{DB: dbConn, Worker: asynq})
	notificationpb.RegisterNotificationHandlersServer(grpcServer, &notification.Handler{DB: dbConn, Worker: asynq})
	userpb.RegisterUserHandlersServer(grpcServer, &user.Handler{DB: dbConn, Worker: asynq})
	licensepb.RegisterLicenseHandlersServer(grpcServer, &license.Handler{})
	keypb.RegisterKeyHandlersServer(grpcServer, &key.Handler{DB: dbConn, Redis: redisConn})
	eventpb.RegisterEventHandlersServer(grpcServer, &event.Handler{DB: dbConn})

	// utilitypb.RegisterUtilityHandlersServer(grpcServer, &utility.Handler{DB: dbConn}) // TODO remove
	// infopb.RegisterInfoHandlersServer(grpcServer, &info.Handler{DB: dbConn}) // TODO remove
	systempb.RegisterSystemHandlersServer(grpcServer, &system.Handler{DB: dbConn})

	return grpcServer
}
