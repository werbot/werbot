package license_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	licensemessage "github.com/werbot/werbot/internal/core/license/proto/message"
	licenserpc "github.com/werbot/werbot/internal/core/license/proto/rpc"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/pkg/utils/fsutil"
)

func Test_license(t *testing.T) {
	ctx := context.Background()

	setup, teardownTestCase := test.GRPC(t)
	defer teardownTestCase(t)

	fixturePath := "../../../fixtures/licenses/"
	pubKeyOk := string(fsutil.MustReadFile(fixturePath + "publicKey_ok.key"))
	licenseOk := fixturePath + "license_ok.key"
	licenseErr := fixturePath + "license_err.key"

	testCases := []struct {
		name      string
		licPath   string
		licPubKey string
		req       *licensemessage.License_Request
		resp      *licensemessage.License_Response
		respErr   string
	}{
		{ // License file found
			name:      "test0_01",
			licPath:   licenseOk,
			licPubKey: pubKeyOk,
			req:       &licensemessage.License_Request{},
			resp: &licensemessage.License_Response{
				Customer: "8ED96811-1804-4A13-9CE7-05874869A1CF",
				Type:     "Enterprise",
				Modules:  []string{"module1", "module2", "module3"},
				Limits: map[string]int32{
					"Companies": 99,
					"Schemes":   99,
					"Users":     99,
				},
				Expired: false,
			},
		},
		{ // License file found and no public key (gen os lic)
			name:      "test0_02",
			licPath:   licenseOk,
			licPubKey: "",
			req:       &licensemessage.License_Request{},
			resp: &licensemessage.License_Response{
				Customer: "Mr. Robot",
				Type:     "open source",
				Modules:  []string{"module1", "module2", "module3"},
				Limits: map[string]int32{
					"Companies": 99,
					"Schemes":   99,
					"Users":     99,
				},
				Expired: true,
			},
		},
		{ // License file not found (gen os lic)
			name:      "test0_03",
			licPath:   "/license.key",
			licPubKey: pubKeyOk,
			req:       &licensemessage.License_Request{},
			resp: &licensemessage.License_Response{
				Customer: "Mr. Robot",
				Type:     "open source",
				Modules:  []string{"module1", "module2", "module3"},
				Limits: map[string]int32{
					"Companies": 99,
					"Schemes":   99,
					"Users":     99,
				},
				Expired: true,
			},
		},
		{ // License file found but is broken
			name:      "test0_04",
			licPath:   licenseErr,
			licPubKey: pubKeyOk,
			req:       &licensemessage.License_Request{},
			resp:      &licensemessage.License_Response{},
			respErr:   "rpc error: code = PermissionDenied desc = The license has a broken",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LICENSE_FILE", tt.licPath)
			t.Setenv("LICENSE_KEY_PUBLIC", tt.licPubKey)

			l := licenserpc.NewLicenseHandlersClient(setup)
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
