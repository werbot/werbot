package license

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/werbot/werbot/internal"
	licensemessage "github.com/werbot/werbot/internal/core/license/proto/message"
	"github.com/werbot/werbot/internal/trace"
	license_lib "github.com/werbot/werbot/pkg/license"
	"github.com/werbot/werbot/pkg/uuid"
)

// errLicenseDataIncomplete indicates that the license data is missing required fields.
var errLicenseDataIncomplete = errors.New("license data is incomplete")

// licenseData is the typed payload of the "dat" license field.
type licenseData struct {
	Companies *int32   `json:"companies"`
	Schemes   *int32   `json:"schemes"`
	Users     *int32   `json:"users"`
	Modules   []string `json:"modules"`
}

// License is ...
func (h *Handler) License(_ context.Context, in *licensemessage.License_Request) (*licensemessage.License_Response, error) {
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

func osLicense() *licensemessage.License_Response {
	now := time.Now().UTC()

	return &licensemessage.License_Response{
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
			"Schemes":   99,
			"Users":     99,
		},
	}
}

func eeLicense(licByte []byte, publicKey string) (*licensemessage.License_Response, error) {
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

	limits, modules, err := parseLicenseData(licDecode.License.Dat)
	if err != nil {
		errGRPC := status.Error(codes.InvalidArgument, trace.MsgStructureIsBroken)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &licensemessage.License_Response{
		Issued:     licDecode.License.Iss,
		Customer:   licDecode.License.Cus,
		Subscriber: licDecode.License.Sub,
		Type:       licDecode.License.Typ,
		IssuedAt:   timestamppb.New(licDecode.License.Iat),
		ExpiresAt:  timestamppb.New(licDecode.License.Exp),
		Expired:    lic.Expired(),
		Limits:     limits,
		Modules:    modules,
	}, nil
}

// parseLicenseData decodes the license payload without unchecked type assertions.
func parseLicenseData(raw json.RawMessage) (map[string]int32, []string, error) {
	data := &licenseData{}
	if err := json.Unmarshal(raw, data); err != nil {
		return nil, nil, err
	}

	if data.Companies == nil || data.Schemes == nil || data.Users == nil || data.Modules == nil {
		return nil, nil, errLicenseDataIncomplete
	}

	return map[string]int32{
		"Companies": *data.Companies,
		"Schemes":   *data.Schemes,
		"Users":     *data.Users,
	}, data.Modules, nil
}
