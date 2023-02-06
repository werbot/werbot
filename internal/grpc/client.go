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
)

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
