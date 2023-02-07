package utility

import (
	"context"
	"database/sql"
	"net"

	"github.com/oschwald/geoip2-golang"

	"github.com/werbot/werbot/internal"
	utilitypb "github.com/werbot/werbot/internal/grpc/utility/proto"
)

// Countries is searches for a country by first letters
func (h *Handler) Countries(ctx context.Context, in *utilitypb.Countries_Request) (*utilitypb.Countries_Response, error) {
	response := new(utilitypb.Countries_Response)

	rows, err := h.DB.Conn.Query(`SELECT "code", "name" FROM "country"
    WHERE LOWER("name") LIKE LOWER($1)
    ORDER BY "name" ASC LIMIT 15 OFFSET 0`,
		in.GetName()+"%",
	)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	for rows.Next() {
		country := new(utilitypb.Countries_Country)
		err = rows.Scan(
			&country.Code,
			&country.Name,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			log.FromGRPC(err).Send()
			return nil, errServerError
		}

		response.Countries = append(response.Countries, country)
	}
	defer rows.Close()

	return response, nil
}

// CountryByIP is determines the country by IP
func (h *Handler) CountryByIP(ctx context.Context, in *utilitypb.CountryByIP_Request) (*utilitypb.CountryByIP_Response, error) {
	response := new(utilitypb.CountryByIP_Response)

	db, err := geoip2.Open(internal.GetString("SECURITY_GEOIP2", "/etc/geoip2/GeoLite2-Country.mmdb"))
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToOpenFile
	}
	defer db.Close()

	record, err := db.City(net.ParseIP(in.GetIp()))
	response.Name = record.Country.Names["en"]
	response.Code = record.Country.IsoCode
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errAccessIsDenied
	}

	return response, nil
}
