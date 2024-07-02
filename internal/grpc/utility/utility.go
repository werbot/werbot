package utility

import (
	"context"
	"net"

	"github.com/oschwald/geoip2-golang"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal"
	utilitypb "github.com/werbot/werbot/internal/grpc/utility/proto"
	"github.com/werbot/werbot/internal/trace"
)

// Countries is searches for a country by first letters
func (h *Handler) Countries(ctx context.Context, in *utilitypb.Countries_Request) (*utilitypb.Countries_Response, error) {
	response := &utilitypb.Countries_Response{}

	rows, err := h.DB.Conn.QueryContext(ctx, `
    SELECT
      "code",
      "name"
    FROM "country"
    WHERE LOWER("name") LIKE LOWER($1)
    ORDER BY "name" ASC
    LIMIT 15
    OFFSET 0
  `, in.GetName()+"%")
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	for rows.Next() {
		country := &utilitypb.Countries_Country{}
		err = rows.Scan(
			&country.Code,
			&country.Name,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		response.Countries = append(response.Countries, country)
	}
	defer rows.Close()

	return response, nil
}

// CountryByIP is determines the country by IP
func (h *Handler) CountryByIP(ctx context.Context, in *utilitypb.CountryByIP_Request) (*utilitypb.CountryByIP_Response, error) {
	response := &utilitypb.CountryByIP_Response{}

	db, err := geoip2.Open(internal.GetString("SECURITY_GEOIP2", "/etc/geoip2/GeoLite2-Country.mmdb"))
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToOpenFile)
	}
	defer db.Close()

	record, err := db.City(net.ParseIP(in.GetIp()))
	response.Name = record.Country.Names["en"]
	response.Code = record.Country.IsoCode
	if err != nil {
		// return nil, errAccessIsDenied
		errGRPC := status.Error(codes.PermissionDenied, trace.MsgAccessIsDeniedCountry)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return response, nil
}
