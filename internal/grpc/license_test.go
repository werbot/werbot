package grpc

import (
	"context"
	"log"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	licensepb "github.com/werbot/werbot/api/proto/license"
	"github.com/werbot/werbot/pkg/fsutil"
)

var (
	pubKeyOk   string
	pubKeyErr  string
	licenseOk  string
	licenseErr string
)

func init() {
	fixturePath := "../../fixtures/licenses/"

	pubKeyOk = string(fsutil.MustReadFile(fixturePath + "publicKey_ok.key"))
	pubKeyErr = string(fsutil.MustReadFile(fixturePath + "publicKey_err.key"))
	licenseOk = fixturePath + "license_ok.key"
	licenseErr = fixturePath + "license_err.key"
}

func dialer(t *testing.T) func(context.Context, string) (net.Conn, error) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	licensepb.RegisterLicenseHandlersServer(server, &license{})
	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()

	return func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
}

func checkConnection(ctx context.Context, t *testing.T) *grpc.ClientConn {
	conn, err := grpc.DialContext(ctx, "", grpc.WithInsecure(), grpc.WithContextDialer(dialer(t)))
	require.NoError(t, err)

	return conn
}

func Test_license(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		licPath   string
		licPubKey string
		req       *licensepb.License_Request
		resp      *licensepb.License_Response
		respErr   string
	}{
		{
			name:      "License file found",
			licPath:   licenseOk,
			licPubKey: pubKeyOk,
			req:       &licensepb.License_Request{},
			resp: &licensepb.License_Response{
				Customer: "8ED96811-1804-4A13-9CE7-05874869A1CF",
				Type:     "Enterprise",
				Modules:  []string{"module1", "module2", "module3"},
				Limits: map[string]int32{
					"Companies": 99,
					"Servers":   99,
					"Users":     99,
				},
				Expired: false,
			},
		},
		{
			name:      "License file found and no public key",
			licPath:   licenseOk,
			licPubKey: "",
			req:       &licensepb.License_Request{},
			resp: &licensepb.License_Response{
				Customer: "Mr. Robot",
				Type:     "open source",
				Modules:  []string{"module1", "module2", "module3"},
				Limits: map[string]int32{
					"Companies": 99,
					"Servers":   99,
					"Users":     99,
				},
				Expired: true,
			},
		},
		{
			name:      "License file not found",
			licPath:   "/license.key",
			licPubKey: pubKeyOk,
			req:       &licensepb.License_Request{},
			resp: &licensepb.License_Response{
				Customer: "Mr. Robot",
				Type:     "open source",
				Modules:  []string{"module1", "module2", "module3"},
				Limits: map[string]int32{
					"Companies": 99,
					"Servers":   99,
					"Users":     99,
				},
				Expired: true,
			},
		},
		{
			name:      "License file found but is broken",
			licPath:   licenseErr,
			licPubKey: pubKeyOk,
			req:       &licensepb.License_Request{},
			resp:      &licensepb.License_Response{},
			respErr:   "rpc error: code = Unknown desc = the license has a broken structure",
		},
	}

	ctx := context.Background()
	conn := checkConnection(ctx, t)
	defer conn.Close()

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("LICENSE_FILE", tt.licPath)
			os.Setenv("LICENSE_KEY_PUBLIC", tt.licPubKey)

			l := licensepb.NewLicenseHandlersClient(conn)
			response, err := l.License(ctx, tt.req)
			if err != nil {
				require.EqualError(t, err, tt.respErr)
				return
			}
			require.NoError(t, err)

			require.Equal(t, tt.resp.Customer, response.Customer)
			require.Equal(t, tt.resp.Type, response.Type)
			require.Equal(t, tt.resp.Expired, response.Expired)
			require.Equal(t, tt.resp.Modules, response.Modules)
			require.Equal(t, tt.resp.Limits, response.Limits)
		})
	}
}
