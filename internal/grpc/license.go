package grpc

import (
	"context"
	"encoding/json"
	"os"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/werbot/werbot/internal"
	license_lib "github.com/werbot/werbot/internal/license"

	pb_license "github.com/werbot/werbot/api/proto/license"
)

type license struct {
	pb_license.UnimplementedLicenseHandlersServer
}

// License is ...
func (l *license) License(ctx context.Context, in *pb_license.License_Request) (*pb_license.License_Response, error) {
	readFile, err := os.ReadFile(internal.GetString("LICENSE_FILE", "/license.key"))
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, license_lib.ErrFailedToOpenLicenseFile
	}

	licensePublic := internal.GetString("LICENSE_KEY_PUBLIC", "")
	lic, err := license_lib.Read([]byte(licensePublic))
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, license_lib.ErrLicenseKeyIsBroken
	}

	licDecode, err := lic.Decode(readFile)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, license_lib.ErrLicenseStructureIsBroken
	}

	var licData = new(pb_license.LicenseInfo_Limits)
	err = json.Unmarshal(licDecode.License.Dat, &licData)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, license_lib.ErrLicenseStructureIsBroken
	}

	return &pb_license.License_Response{
		License: &pb_license.LicenseInfo{
			Issued:     licDecode.License.Iss,
			Customer:   licDecode.License.Cus,
			Subscriber: licDecode.License.Sub,
			Type:       licDecode.License.Typ,
			IssuedAt:   timestamppb.New(licDecode.License.Iat),
			ExpiresAt:  timestamppb.New(licDecode.License.Exp),
			Limits: &pb_license.LicenseInfo_Limits{
				Companies: licData.Companies,
				Servers:   licData.Servers,
				Users:     licData.Users,
				Modules:   licData.Modules,
			},
		},
		Expired: lic.Expired(),
	}, nil
}
