package grpc

import (
	"context"

	pb_utility "github.com/werbot/werbot/api/proto/utility"
)

type utility struct {
	pb_utility.UnimplementedUtilityHandlersServer
}

// GetCountry is searches for a country by first letters
func (u *utility) GetCountry(ctx context.Context, in *pb_utility.GetCountry_Request) (*pb_utility.GetCountry_Response, error) {
	countries := []*pb_utility.GetCountry_Response_Country{}
	rows, err := service.db.Conn.Query(`SELECT
      "code",
      "name"
    FROM
      "country"
    WHERE
      LOWER("name") LIKE LOWER($1)
    ORDER BY "name" ASC
		LIMIT 15 OFFSET 0`,
		in.Name+"%",
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToSelect
	}

	for rows.Next() {
		country := new(pb_utility.GetCountry_Response_Country)
		err = rows.Scan(
			&country.Code,
			&country.Name,
		)
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, errFailedToScan
		}

		countries = append(countries, country)
	}

	defer rows.Close()

	return &pb_utility.GetCountry_Response{
		Countries: countries,
	}, nil
}
