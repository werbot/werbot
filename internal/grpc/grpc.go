package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"

	accountpb "github.com/werbot/werbot/api/proto/account"
	auditpb "github.com/werbot/werbot/api/proto/audit"
	authpb "github.com/werbot/werbot/api/proto/auth"
	billingpb "github.com/werbot/werbot/api/proto/billing"
	firewallpb "github.com/werbot/werbot/api/proto/firewall"
	infopb "github.com/werbot/werbot/api/proto/info"
	keypb "github.com/werbot/werbot/api/proto/key"
	licensepb "github.com/werbot/werbot/api/proto/license"
	memberpb "github.com/werbot/werbot/api/proto/member"
	projectpb "github.com/werbot/werbot/api/proto/project"
	serverpb "github.com/werbot/werbot/api/proto/server"
	subscriptionpb "github.com/werbot/werbot/api/proto/subscription"
	userpb "github.com/werbot/werbot/api/proto/user"
	utilitypb "github.com/werbot/werbot/api/proto/utility"

	"github.com/werbot/werbot/internal/storage/cache"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

var service Service

// ServerService is ...
type ServerService struct {
	GRPC *grpc.Server
}

// Service is ...
type Service struct {
	db    *postgres.Connect
	cache cache.Cache
	log   logger.Logger

	token string
}

// NewServer is ...
func NewServer(token string, dbConn *postgres.Connect, cacheConn cache.Cache, cert tls.Certificate) *ServerService {
	log := logger.New("internal/grpc")
	service = Service{
		db:    dbConn,
		cache: cacheConn,
		log:   log,
		token: token,
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewServerTLSFromCert(&cert)),
		grpc.UnaryInterceptor(ensureValidToken),
	)

	authpb.RegisterAuthHandlersServer(grpcServer, &auth{})
	accountpb.RegisterAccountHandlersServer(grpcServer, &account{})
	auditpb.RegisterAuditHandlersServer(grpcServer, &audit{})
	firewallpb.RegisterFirewallHandlersServer(grpcServer, &firewall{})
	serverpb.RegisterServerHandlersServer(grpcServer, &server{})
	projectpb.RegisterProjectHandlersServer(grpcServer, &project{})
	memberpb.RegisterMemberHandlersServer(grpcServer, &member{})
	subscriptionpb.RegisterSubscriptionHandlersServer(grpcServer, &subscription{})
	userpb.RegisterUserHandlersServer(grpcServer, &user{})
	billingpb.RegisterBillingHandlersServer(grpcServer, &billing{})
	licensepb.RegisterLicenseHandlersServer(grpcServer, &license{})
	infopb.RegisterInfoHandlersServer(grpcServer, &info{})
	keypb.RegisterKeyHandlersServer(grpcServer, &key{})
	utilitypb.RegisterUtilityHandlersServer(grpcServer, &utility{})

	return &ServerService{
		GRPC: grpcServer,
	}
}

// ----------------------------------------------------------------------------

// ClientService is ...
type ClientService struct {
	Client   *grpc.ClientConn
	certPool *x509.CertPool
}

// NewClient is ...
func NewClient(dsn, token, nameOverride string, certPEM, keyPem []byte) *ClientService {
	// init certificate setting
	cert, err := tls.X509KeyPair(certPEM, keyPem)
	if err != nil {
		service.log.Fatal(err).Msg("Failed to parse key pair")
	}
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		service.log.Fatal(err).Msg("Failed to parse certificate")
	}

	cfg := &ClientService{
		certPool: x509.NewCertPool(),
	}
	cfg.certPool.AddCert(cert.Leaf)

	// init grpc
	perRPC := oauth.NewOauthAccess(&oauth2.Token{
		AccessToken: token,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, dsn,
		grpc.WithPerRPCCredentials(perRPC),
		grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(cfg.certPool, nameOverride)),
	)
	if err != nil {
		service.log.Error(err).Msg("Failed to dial server")
	}
	// defer conn.Close()

	cfg.Client = conn
	return cfg
}
