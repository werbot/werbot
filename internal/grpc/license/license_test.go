package license_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	licensepb "github.com/werbot/werbot/internal/grpc/license/proto"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/pkg/fsutil"
)

func Test_license(t *testing.T) {
	// t.Parallel()

	ctx := context.Background()
	grpc, _ := test.GRPC(ctx, t, nil, nil)
	defer grpc.Close()

	fixturePath := "../../../fixtures/licenses/"
	pubKeyOk := string(fsutil.MustReadFile(fixturePath + "publicKey_ok.key"))
	licenseOk := fixturePath + "license_ok.key"
	licenseErr := fixturePath + "license_err.key"

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
			resp:      &licensepb.License_Response{},
			respErr:   "rpc error: code = Unknown desc = Failed to open license file",
		},
		{
			name:      "License file found but is broken",
			licPath:   licenseErr,
			licPubKey: pubKeyOk,
			req:       &licensepb.License_Request{},
			resp:      &licensepb.License_Response{},
			respErr:   "rpc error: code = Unknown desc = The license has a broken",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LICENSE_FILE", tt.licPath)
			t.Setenv("LICENSE_KEY_PUBLIC", tt.licPubKey)

			l := licensepb.NewLicenseHandlersClient(grpc)
			response, err := l.License(ctx, tt.req)
			if err != nil {
				assert.EqualError(t, err, tt.respErr)
				return
			}
			assert.NoError(t, err)

			assert.Equal(t, tt.resp.Customer, response.Customer)
			assert.Equal(t, tt.resp.Type, response.Type)
			assert.Equal(t, tt.resp.Expired, response.Expired)
			assert.Equal(t, tt.resp.Modules, response.Modules)
			assert.Equal(t, tt.resp.Limits, response.Limits)
		})
	}
}
