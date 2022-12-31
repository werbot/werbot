package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb_account "github.com/werbot/werbot/api/proto/account"
	pb_audit "github.com/werbot/werbot/api/proto/audit"
	pb_billing "github.com/werbot/werbot/api/proto/billing"
	pb_firewall "github.com/werbot/werbot/api/proto/firewall"
	pb_info "github.com/werbot/werbot/api/proto/info"
	pb_key "github.com/werbot/werbot/api/proto/key"
	pb_license "github.com/werbot/werbot/api/proto/license"
	pb_member "github.com/werbot/werbot/api/proto/member"
	pb_project "github.com/werbot/werbot/api/proto/project"
	pb_server "github.com/werbot/werbot/api/proto/server"
	pb_subscription "github.com/werbot/werbot/api/proto/subscription"
	pb_update "github.com/werbot/werbot/api/proto/update"
	pb_user "github.com/werbot/werbot/api/proto/user"
	pb_utility "github.com/werbot/werbot/api/proto/utility"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/storage/cache"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

var service Service

var (
	errNotFound               = errors.New(internal.MsgNotFound)
	errBadRequest             = errors.New(internal.MsgBadRequest)
	errIncorrectParameters    = errors.New(internal.MsgIncorrectParameters)
	errPasswordIsNotValid     = errors.New(internal.MsgPasswordIsNotValid)
	errAccessIsDenied         = errors.New(internal.MsgAccessIsDenied)
	errAccessIsDeniedUser     = errors.New(internal.MsgAccessIsDeniedUser)
	errAccessIsDeniedTime     = errors.New(internal.MsgAccessIsDeniedTime)
	errAccessIsDeniedCountry  = errors.New(internal.MsgAccessIsDeniedCountry)
	errAccessIsDeniedIP       = errors.New(internal.MsgAccessIsDeniedIP)
	errObjectAlreadyExists    = errors.New(internal.MsgObjectAlreadyExists)
	errFailedToOpenFile       = errors.New(internal.MsgFailedToOpenFile)
	errFailedToSelect         = errors.New(internal.MsgFailedToSelect)
	errFailedToAdd            = errors.New(internal.MsgFailedToAdd)
	errFailedToUpdate         = errors.New(internal.MsgFailedToUpdate)
	errFailedToDelete         = errors.New(internal.MsgFailedToDelete)
	errFailedToScan           = errors.New(internal.MsgFailedToScan)
	errInviteIsInvalid        = errors.New(internal.MsgInviteIsInvalid)
	errInviteIsActivated      = errors.New(internal.MsgInviteIsActivated)
	errTransactionCreateError = errors.New(internal.MsgTransactionCreateError)
	errTransactionCommitError = errors.New(internal.MsgTransactionCreateError)
	errTokenIsNotValid        = errors.New(internal.MsgTokenIsNotValid)
	errHashIsNotValid         = errors.New(internal.MsgHashIsNotValid)
)

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

	pb_account.RegisterAccountHandlersServer(grpcServer, &account{})
	pb_audit.RegisterAuditHandlersServer(grpcServer, &audit{})
	pb_firewall.RegisterFirewallHandlersServer(grpcServer, &firewall{})
	pb_server.RegisterServerHandlersServer(grpcServer, &server{})
	pb_project.RegisterProjectHandlersServer(grpcServer, &project{})
	pb_member.RegisterMemberHandlersServer(grpcServer, &member{})
	pb_subscription.RegisterSubscriptionHandlersServer(grpcServer, &subscription{})
	pb_user.RegisterUserHandlersServer(grpcServer, &user{})
	pb_billing.RegisterBillingHandlersServer(grpcServer, &billing{})
	pb_license.RegisterLicenseHandlersServer(grpcServer, &license{})
	pb_info.RegisterInfoHandlersServer(grpcServer, &info{})
	pb_key.RegisterKeyHandlersServer(grpcServer, &key{})
	pb_update.RegisterUpdateHandlersServer(grpcServer, &update{})
	pb_utility.RegisterUtilityHandlersServer(grpcServer, &utility{})

	return &ServerService{
		GRPC: grpcServer,
	}
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

func valid(authorization []string) bool {
	if len(authorization) < 1 {
		return false
	}
	token := strings.TrimPrefix(authorization[0], "Bearer ")
	return token == service.token
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
