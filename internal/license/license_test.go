package license

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/werbot/werbot/pkg/fsutil"
)

var (
	privKeyOk  []byte
	privKeyErr []byte
	pubKeyOk   []byte
	pubKeyErr  []byte
	licenseOk  []byte
	licenseExp []byte
	licenseErr []byte
)

func init() {
	fixturePath := "../../fixtures/licenses/"

	privKeyOk = fsutil.MustReadFile(fixturePath + "privateKey_ok.key")
	privKeyErr = fsutil.MustReadFile(fixturePath + "privateKey_err.key")
	pubKeyOk = fsutil.MustReadFile(fixturePath + "publicKey_ok.key")
	pubKeyErr = fsutil.MustReadFile(fixturePath + "publicKey_err.key")
	licenseOk = fsutil.MustReadFile(fixturePath + "license_ok.key")
	licenseExp = fsutil.MustReadFile(fixturePath + "license_exp.key")
	licenseErr = fsutil.MustReadFile(fixturePath + "license_err.key")
}

func TestDecodePrivateKey(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		privateKey []byte
		respErr    string
	}{
		{
			name:       "Decode private key",
			privateKey: privKeyOk,
		},
		{
			name:       "Decode broke private key",
			privateKey: privKeyErr,
			respErr:    "illegal base64 data",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodePrivateKey(tt.privateKey)
			if err != nil {
				require.EqualError(t, err, tt.respErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestLicenseCreate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		privateKey []byte
		publicKey  []byte
		license    License
		respErr    string
	}{
		{
			name:       "Create new license",
			privateKey: privKeyOk,
			publicKey:  pubKeyOk,
			license: License{
				Iss: "1",
				Cus: "2",
				Sub: "3",
				Typ: "4",
				Ips: "5",
				Iat: time.Now().UTC(),
				Exp: time.Now().UTC(),
				Dat: json.RawMessage(nil),
			},
		},
		{
			name:       "Broken private key",
			privateKey: privKeyErr,
			publicKey:  pubKeyOk,
			respErr:    "illegal base64 data",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			forDecode, err := DecodePrivateKey(tt.privateKey) // for generate license
			if err != nil {
				require.EqualError(t, err, tt.respErr)
				return
			}
			forDecode.License = tt.license

			forEncode, err := DecodePublicKey(tt.publicKey) // for test read generated license
			if err != nil {
				require.EqualError(t, err, tt.respErr)
				return
			}

			license, err := forDecode.Encode()
			if err != nil {
				require.EqualError(t, err, tt.respErr)
				return
			}

			data, err := forEncode.Decode(license)
			if err != nil {
				require.EqualError(t, err, tt.respErr)
				return
			}
			require.NoError(t, err)

			require.Equal(t, tt.license, data.License)
		})
	}
}

func TestLicenseRead(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		publicKey []byte
		license   []byte
		response  License
		expired   bool
		respErr   string
	}{
		{
			name:      "No expires license",
			publicKey: pubKeyOk,
			license:   licenseOk,
			response: License{
				Iss: "Werbot, Inc.",
				Cus: "8ED96811-1804-4A13-9CE7-05874869A1CF",
				Sub: "EED1CA19-4DC5-4376-83F5-61077B501961",
				Typ: "Enterprise",
				Ips: "127.0.0.1",
				Iat: time.Date(2023, time.January, 10, 16, 55, 43, 277033166, time.UTC),
				Exp: time.Date(2050, time.May, 27, 16, 55, 43, 277033272, time.UTC),
				Dat: json.RawMessage([]byte(`{"servers":99,"companies":99,"users":99,"modules":["module1","module2","module3"]}`)),
			},
			expired: false,
		},
		{
			name:      "Expires license",
			publicKey: pubKeyOk,
			license:   licenseExp,
			response: License{
				Iss: "Werbot, Inc.",
				Cus: "8ED96811-1804-4A13-9CE7-05874869A1CF",
				Sub: "EED1CA19-4DC5-4376-83F5-61077B501961",
				Typ: "Enterprise",
				Ips: "127.0.0.1",
				Iat: time.Date(2023, time.January, 10, 16, 57, 16, 390218045, time.UTC),
				Exp: time.Date(2023, time.January, 10, 16, 57, 16, 390218129, time.UTC),
				Dat: json.RawMessage([]byte(`{"servers":99,"companies":99,"users":99,"modules":["module1","module2","module3"]}`)),
			},
			expired: true,
		},
		{
			name:      "Read broken license",
			publicKey: pubKeyOk,
			license:   licenseErr,
			respErr:   "invalid license",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			forEncode, err := DecodePublicKey(tt.publicKey)
			if err != nil {
				require.EqualError(t, err, tt.respErr)
				return
			}

			data, err := forEncode.Decode(tt.license)
			if err != nil {
				require.EqualError(t, err, tt.respErr)
				return
			}
			require.NoError(t, err)

			require.Equal(t, tt.response, data.Info())
			require.Equal(t, tt.expired, data.Expired())
		})
	}
}
