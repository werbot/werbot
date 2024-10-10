package system

import (
	"context"
	"net"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/oschwald/geoip2-golang"
	"github.com/werbot/werbot/internal"
	systempb "github.com/werbot/werbot/internal/core/system/proto/system"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/utils/protoutils"
)

// Countries is searches for a country by first letters
func (h *Handler) Countries(ctx context.Context, in *systempb.Countries_Request) (*systempb.Countries_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &systempb.Countries_Response{}

	rows, err := h.DB.Conn.QueryContext(ctx, `
    SELECT
      "code",
      "name"
    FROM "country"
    WHERE LOWER("name") LIKE LOWER($1)
    ORDER BY "name" ASC
    LIMIT 15 OFFSET 0
  `, in.GetName()+"%")
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		country := &systempb.Countries_Country{}
		err = rows.Scan(
			&country.Code,
			&country.Name,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		response.Countries = append(response.Countries, country)
	}

	if len(response.Countries) == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return response, nil
}

// CountryByIP is determines the country by IP
func (h *Handler) CountryByIP(_ context.Context, in *systempb.CountryByIP_Request) (*systempb.CountryByIP_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	db, err := geoip2.Open(internal.GetString("SECURITY_GEOIP2", "/etc/geoip2/GeoLite2-Country.mmdb"))
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToOpenFile)
	}
	defer db.Close()

	record, err := db.Country(net.ParseIP(in.GetIp()))
	response := &systempb.CountryByIP_Response{
		Name: record.Country.Names["en"],
		Code: record.Country.IsoCode,
	}
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgAborted)
	}

	return response, nil
}
