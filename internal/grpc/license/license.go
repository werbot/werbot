package license

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/werbot/werbot/internal"
	licensepb "github.com/werbot/werbot/internal/grpc/license/proto/license"
	"github.com/werbot/werbot/internal/trace"
	license_lib "github.com/werbot/werbot/pkg/license"
	"github.com/werbot/werbot/pkg/uuid"
)

// License is ...
func (h *Handler) License(_ context.Context, in *licensepb.License_Request) (*licensepb.License_Response, error) {
	licenseFilePath := internal.GetString("LICENSE_FILE", "/license.key")
	licensePublicKey := internal.GetString("LICENSE_KEY_PUBLIC", "")

	licByte := in.GetLicense()
	if licByte == nil {
		var err error
		licByte, err = os.ReadFile(licenseFilePath)
		if err != nil || licensePublicKey == "" {
			return osLicense(), nil
		}
	}

	return eeLicense(licByte, licensePublicKey)
}

func osLicense() *licensepb.License_Response {
	now := time.Now().UTC()

	return &licensepb.License_Response{
		Issued:     "free",
		Customer:   "Mr. Robot",
		Subscriber: uuid.New(),
		Type:       "open source",
		IssuedAt:   timestamppb.New(now),
		ExpiresAt:  timestamppb.New(now.AddDate(1, 0, 0)),
		Expired:    true,
		Modules:    []string{"module1", "module2", "module3"},
		Limits: map[string]int32{
			"Companies": 99,
			"Servers":   99,
			"Users":     99,
		},
	}
}

func eeLicense(licByte []byte, publicKey string) (*licensepb.License_Response, error) {
	lic, err := license_lib.DecodePublicKey([]byte(publicKey))
	if err != nil {
		errGRPC := status.Error(codes.NotFound, trace.MsgLicenseKeyIsBroken)
		return nil, trace.Error(errGRPC, log, nil)
	}

	licDecode, err := lic.Decode(licByte)
	if err != nil {
		errGRPC := status.Error(codes.PermissionDenied, trace.MsgLicenseStructureIsBroken)
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &licensepb.License_Response{
		Issued:     licDecode.License.Iss,
		Customer:   licDecode.License.Cus,
		Subscriber: licDecode.License.Sub,
		Type:       licDecode.License.Typ,
		IssuedAt:   timestamppb.New(licDecode.License.Iat),
		ExpiresAt:  timestamppb.New(licDecode.License.Exp),
		Expired:    lic.Expired(),
	}

	licData := map[string]any{}
	if err := json.Unmarshal(licDecode.License.Dat, &licData); err != nil {
		return nil, trace.Error(err, log, trace.MsgStructureIsBroken)
	}

	response.Limits = map[string]int32{
		"Companies": int32(licData["companies"].(float64)),
		"Servers":   int32(licData["servers"].(float64)),
		"Users":     int32(licData["users"].(float64)),
	}

	for _, item := range licData["modules"].([]interface{}) {
		response.Modules = append(response.Modules, item.(string))
	}

	return response, nil
}
