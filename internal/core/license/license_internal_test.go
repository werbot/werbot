package license

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	license_lib "github.com/werbot/werbot/pkg/license"
	"github.com/werbot/werbot/pkg/utils/fsutil"
)

// signedLicense builds a validly signed license with an arbitrary "dat" payload.
func signedLicense(t *testing.T, data string) []byte {
	t.Helper()

	fixturePath := "../../../fixtures/licenses/"
	privateKey := fsutil.MustReadFile(fixturePath + "privateKey_ok.key")

	lic, err := license_lib.DecodePrivateKey(privateKey)
	require.NoError(t, err)

	now := time.Now().UTC()
	lic.License = license_lib.License{
		Iss: "Werbot, Inc.",
		Cus: "CUSTOMER",
		Sub: "SUBSCRIBER",
		Typ: "Enterprise",
		Iat: now,
		Exp: now.AddDate(1, 0, 0),
		Dat: json.RawMessage(data),
	}

	encoded, err := lic.Encode()
	require.NoError(t, err)

	return encoded
}

func Test_eeLicense_invalidData(t *testing.T) {
	fixturePath := "../../../fixtures/licenses/"
	publicKey := string(fsutil.MustReadFile(fixturePath + "publicKey_ok.key"))

	testCases := []struct {
		name string
		data string
	}{
		{
			name: "missing limits and modules",
			data: `{}`,
		},
		{
			name: "missing modules",
			data: `{"companies": 1, "schemes": 2, "users": 3}`,
		},
		{
			name: "wrong types",
			data: `{"companies": "1", "schemes": 2, "users": 3, "modules": ["m"]}`,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			response, err := eeLicense(signedLicense(t, tt.data), publicKey)

			assert.Nil(t, response)
			require.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}
