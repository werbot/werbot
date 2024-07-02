package grpc

import (
	"crypto/tls"
	"crypto/x509"

	"github.com/werbot/werbot/internal"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
)

// NewClient is ...
func NewClient() (*grpc.ClientConn, error) {
	dsn := internal.GetString("GRPCSERVER_HOST", "localhost:50051")
	token := internal.GetString("GRPCSERVER_TOKEN", "token")
	nameOverride := internal.GetString("GRPCSERVER_NAMEOVERRIDE", "werbot.com")

	// init certificate setting
	certPEM, err := internal.GetByteFromFile("GRPCSERVER_CERTIFICATE", "./grpc_certificate.key")
	if err != nil {
		return nil, err // Failed to open grpc_certificate.key
	}

	keyPem, err := internal.GetByteFromFile("GRPCSERVER_PRIVATE_KEY", "./grpc_private.key")
	if err != nil {
		return nil, err // Failed to open grpc_private.key
	}

	cert, err := tls.X509KeyPair(certPEM, keyPem)
	if err != nil {
		return nil, err // Failed to parse key pair
	}
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, err // Failed to parse certificate
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(cert.Leaf)

	perRPC := oauth.TokenSource{
		TokenSource: oauth2.StaticTokenSource(&oauth2.Token{
			AccessToken: token,
		}),
	}

	// ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	// defer cancel()

	conn, err := grpc.NewClient(dsn,
		grpc.WithPerRPCCredentials(perRPC),
		grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(certPool, nameOverride)))
	if err != nil {
		return nil, err // Failed to dial server
	}
	// defer conn.Close()

	return conn, nil
}
