package grpc

import (
	"context"
	"encoding/json"
	"os"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/werbot/werbot/internal/config"
	license_lib "github.com/werbot/werbot/internal/license"

	pb_license "github.com/werbot/werbot/internal/grpc/proto/license"
)

type license struct {
	pb_license.UnimplementedLicenseHandlersServer
}

// GetLicenseInfo is ...
func (l *license) GetLicenseInfo(ctx context.Context, in *pb_license.GetLicenseInfo_Request) (*pb_license.GetLicenseInfo_Response, error) {
	readFile, err := os.ReadFile(config.GetString("LICENSE_FILE", "/license.key"))
	if err != nil {
		return nil, err
	}

	licensePublic := config.GetString("LICENSE_KEY_PUBLIC", "")
	lic, err := license_lib.GetLicense([]byte(licensePublic))
	if err != nil {
		return nil, err
	}

	licDecode, err := lic.Decode(readFile)
	if err != nil {
		return nil, err
	}

	var licData *pb_license.License_Limits
	if err := json.Unmarshal(licDecode.License.Dat, &licData); err != nil {
		return nil, err
	}

	return &pb_license.GetLicenseInfo_Response{
		License: &pb_license.License{
			Issued:     licDecode.License.Iss,
			Customer:   licDecode.License.Cus,
			Subscriber: licDecode.License.Sub,
			Type:       licDecode.License.Typ,
			IssuedAt:   timestamppb.New(licDecode.License.Iat),
			ExpiresAt:  timestamppb.New(licDecode.License.Exp),
			Limits: &pb_license.License_Limits{
				Companies: licData.Companies,
				Servers:   licData.Servers,
				Users:     licData.Users,
				Modules:   licData.Modules,
			},
		},
		Expired: lic.Expired(),
	}, nil
}
