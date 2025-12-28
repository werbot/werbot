package grpc

import (
	"crypto/tls"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/werbot/werbot/internal/core/agent"
	agentpb "github.com/werbot/werbot/internal/core/agent/proto/rpc"
	"github.com/werbot/werbot/internal/core/audit"
	auditpb "github.com/werbot/werbot/internal/core/audit/proto/rpc"
	"github.com/werbot/werbot/internal/core/event"
	eventpb "github.com/werbot/werbot/internal/core/event/proto/rpc"
	"github.com/werbot/werbot/internal/core/firewall"
	firewallpb "github.com/werbot/werbot/internal/core/firewall/proto/rpc"
	"github.com/werbot/werbot/internal/core/key"
	keyrpc "github.com/werbot/werbot/internal/core/key/proto/rpc"
	"github.com/werbot/werbot/internal/core/license"
	licenserpc "github.com/werbot/werbot/internal/core/license/proto/rpc"
	"github.com/werbot/werbot/internal/core/member"
	memberrpc "github.com/werbot/werbot/internal/core/member/proto/rpc"
	"github.com/werbot/werbot/internal/core/notification"
	notificationrpc "github.com/werbot/werbot/internal/core/notification/proto/rpc"
	"github.com/werbot/werbot/internal/core/profile"
	profilerpc "github.com/werbot/werbot/internal/core/profile/proto/rpc"
	"github.com/werbot/werbot/internal/core/project"
	projectpb "github.com/werbot/werbot/internal/core/project/proto/rpc"
	"github.com/werbot/werbot/internal/core/scheme"
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/core/system"
	systempb "github.com/werbot/werbot/internal/core/system/proto/rpc"
	"github.com/werbot/werbot/internal/core/token"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
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
	agentpb.RegisterAgentHandlersServer(grpcServer, &agent.Handler{DB: dbConn, Redis: redisConn})
	auditpb.RegisterAuditHandlersServer(grpcServer, &audit.Handler{DB: dbConn})
	firewallpb.RegisterFirewallHandlersServer(grpcServer, &firewall.Handler{DB: dbConn})
	tokenpb.RegisterTokenHandlersServer(grpcServer, &token.Handler{DB: dbConn, Worker: asynq})
	schemepb.RegisterSchemeHandlersServer(grpcServer, &scheme.Handler{DB: dbConn, Redis: redisConn})
	projectpb.RegisterProjectHandlersServer(grpcServer, &project.Handler{DB: dbConn})
	memberrpc.RegisterMemberHandlersServer(grpcServer, &member.Handler{DB: dbConn, Worker: asynq})
	notificationrpc.RegisterNotificationHandlersServer(grpcServer, &notification.Handler{DB: dbConn, Worker: asynq})
	profilerpc.RegisterProfileHandlersServer(grpcServer, &profile.Handler{DB: dbConn, Worker: asynq})
	licenserpc.RegisterLicenseHandlersServer(grpcServer, &license.Handler{})
	keyrpc.RegisterKeyHandlersServer(grpcServer, &key.Handler{DB: dbConn, Redis: redisConn})
	eventpb.RegisterEventHandlersServer(grpcServer, &event.Handler{DB: dbConn})

	systempb.RegisterSystemHandlersServer(grpcServer, &system.Handler{DB: dbConn})

	return grpcServer
}
