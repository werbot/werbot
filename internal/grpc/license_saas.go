//go:build saas

package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"

	"github.com/werbot/werbot/internal"
	license_lib "github.com/werbot/werbot/internal/license"

	pb_license "github.com/werbot/werbot/api/proto/license"
)

// NewLicense is ...
func (l *license) NewLicense(ctx context.Context, in *pb_license.NewLicense_Request) (*pb_license.NewLicense_Response, error) {
	var status string
	var licServer []byte
	newLicense := new(pb_license.NewLicense_Response)
	err := service.db.Conn.QueryRow(`SELECT
				"status",
				"license"
			FROM
				"license"
			WHERE
				"ip" = $1::inet`,
		in.GetIp(),
	).Scan(&status, &licServer)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToScan
	}

	if status == "" {
		lic, err := license_lib.New([]byte(internal.GetString("LICENSE_KEY_PRIVATE", "")))
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, license_lib.ErrLicenseKeyIsBroken
		}

		var typeID, period, companies, servers, users int32
		var name string
		var modulesJSON pgtype.JSON
		err = service.db.Conn.QueryRow(`SELECT
				"id",
				"name",
				"period",
				"companies",
				"servers",
				"users",
				"modules"
			FROM
				"license_type"
			WHERE
				"default" = true`).
			Scan(&typeID,
				&name,
				&period,
				&companies,
				&servers,
				&users,
				&modulesJSON,
			)
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, errFailedToScan
		}

		var modules []string
		modulesJSON.AssignTo(&modules)

		now := time.Now()
		licData := &pb_license.LicenseInfo_Limits{
			Companies: companies,
			Servers:   servers,
			Users:     users,
			Modules:   modules,
		}
		licDataByte, err := json.Marshal(licData)
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, license_lib.ErrLicenseStructureIsBroken
		}

		customer := checkUUIDLicenseParam(in.GetCustomer())
		subscriber := checkUUIDLicenseParam(in.GetSubscriber())

		lic.License = &license_lib.License{
			Iss: fmt.Sprintf("Werbot_%s, Inc.", time.Now().Format("20060102150405")),
			Typ: name,
			Cus: customer,
			Sub: uuid.New().String(),
			Ips: in.GetIp(),
			Iat: now.UTC(),
			Exp: now.AddDate(0, 0, int(period)).UTC(),
			//Dat: []byte(`{"servers":200, "companies":20, "users":50, "modules":["success", "error", "warning"]}`),
			Dat: licDataByte,
		}

		licenseByte, err := lic.Encode()
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, license_lib.ErrLicenseStructureIsBroken
		}

		status = "trial"
		data, err := service.db.Conn.Exec(`INSERT
			INTO "public"."license" (
				"version",
				"customer_id",
				"subscriber_id",
				"type_id",
				"ip",
				"status",
				"issued_at",
				"expires_at",
				"license"
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			1,
			customer,
			subscriber,
			typeID,
			in.GetIp(),
			status,
			lic.License.Iat,
			lic.License.Exp,
			licenseByte,
		)
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, errFailedToAdd
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errFailedToAdd
		}

		newLicense.License = licenseByte
		return newLicense, nil
	}

	newLicense.License = licServer
	return newLicense, nil
}

// LicenseExpired is ...
func (l *license) LicenseExpired(ctx context.Context, in *pb_license.LicenseExpired_Request) (*pb_license.LicenseExpired_Response, error) {
	lic, err := license_lib.Read([]byte(internal.GetString("LICENSE_KEY_PUBLIC", "")))
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, license_lib.ErrLicenseKeyIsBroken
	}

	ld, err := lic.Decode(in.GetLicense())
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, license_lib.ErrLicenseStructureIsBroken
	}

	return &pb_license.LicenseExpired_Response{
		Status: ld.Expired(),
	}, nil
}

func checkUUIDLicenseParam(param string) string {
	if len(param) > 0 {
		return param
	}
	return uuid.New().String()
}
