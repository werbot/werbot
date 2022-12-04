package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb_account "github.com/werbot/werbot/internal/grpc/proto/account"
	pb_audit "github.com/werbot/werbot/internal/grpc/proto/audit"
	pb_billing "github.com/werbot/werbot/internal/grpc/proto/billing"
	pb_firewall "github.com/werbot/werbot/internal/grpc/proto/firewall"
	pb_info "github.com/werbot/werbot/internal/grpc/proto/info"
	pb_key "github.com/werbot/werbot/internal/grpc/proto/key"
	pb_license "github.com/werbot/werbot/internal/grpc/proto/license"
	pb_member "github.com/werbot/werbot/internal/grpc/proto/member"
	pb_project "github.com/werbot/werbot/internal/grpc/proto/project"
	pb_server "github.com/werbot/werbot/internal/grpc/proto/server"
	pb_subscription "github.com/werbot/werbot/internal/grpc/proto/subscription"
	pb_update "github.com/werbot/werbot/internal/grpc/proto/update"
	pb_user "github.com/werbot/werbot/internal/grpc/proto/user"
	pb_utility "github.com/werbot/werbot/internal/grpc/proto/utility"

	"github.com/werbot/werbot/internal/logger"
	cache_lib "github.com/werbot/werbot/internal/storage/cache"
	"github.com/werbot/werbot/internal/storage/postgres"
)

var (
	log = logger.New("internal/grpc")

	db     *postgres.Connect
	cache  cache_lib.Cache
	gToken string
)

// ServerService is ...
type ServerService struct {
	GRPC *grpc.Server
}

// NewServer is ...
func NewServer(token string, dbConn *postgres.Connect, cacheConn cache_lib.Cache, cert tls.Certificate) *ServerService {
	gToken = token
	db = dbConn
	cache = cacheConn

	s := grpc.NewServer(
		grpc.Creds(credentials.NewServerTLSFromCert(&cert)),
		grpc.UnaryInterceptor(ensureValidToken),
	)

	pb_account.RegisterAccountHandlersServer(s, &account{})
	pb_audit.RegisterAuditHandlersServer(s, &audit{})
	pb_firewall.RegisterFirewallHandlersServer(s, &firewall{})
	pb_server.RegisterServerHandlersServer(s, &server{})
	pb_project.RegisterProjectHandlersServer(s, &project{})
	pb_member.RegisterMemberHandlersServer(s, &member{})
	pb_subscription.RegisterSubscriptionHandlersServer(s, &subscription{})
	pb_user.RegisterUserHandlersServer(s, &user{})
	pb_billing.RegisterBillingHandlersServer(s, &billing{})
	pb_license.RegisterLicenseHandlersServer(s, &license{})
	pb_info.RegisterInfoHandlersServer(s, &info{})
	pb_key.RegisterKeyHandlersServer(s, &key{})
	pb_update.RegisterUpdateHandlersServer(s, &update{})
	pb_utility.RegisterUtilityHandlersServer(s, &utility{})

	return &ServerService{
		GRPC: s,
	}
}

func valid(authorization []string) bool {
	if len(authorization) < 1 {
		return false
	}
	token := strings.TrimPrefix(authorization[0], "Bearer ")
	return token == gToken
}

func ensureValidToken(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "Missing metadata")
	}

	if !valid(md["authorization"]) {
		return nil, status.Errorf(codes.Unauthenticated, "Token is invalid")
	}
	return handler(ctx, req)
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
		log.Fatal().Msgf("Failed to parse key pair: %s", err)
	}
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		log.Fatal().Msgf("Failed to parse certificate: %s", err)
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
		log.Error().Err(err).Msg("Failed to dial server")
	}
	// defer conn.Close()

	cfg.Client = conn
	return cfg
}
