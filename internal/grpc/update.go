package grpc

import (
	"context"

	"github.com/jackc/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb_update "github.com/werbot/werbot/api/proto/update"
)

type update struct {
	pb_update.UnimplementedUpdateHandlersServer
}

// Update is ...
func (u *update) Update(ctx context.Context, in *pb_update.Update_Request) (*pb_update.Update_Response, error) {
	rows, err := service.db.Conn.Query(`SELECT
			"component",
			"description",
			"version",
			"version_after",
			"issued_at"
		FROM
			"update"`)
	if err != nil {
		return nil, errFailedToSelect
	}

	components := map[string]*pb_update.Update_Response_Component{}

	for rows.Next() {
		var name string
		var issuedAt pgtype.Timestamp
		component := new(pb_update.Update_Response_Component)
		err = rows.Scan(
			&name,
			&component.Description,
			&component.Version,
			&component.VersionAfter,
			&issuedAt,
		)
		if err != nil {
			return nil, errFailedToScan
		}
		component.IssuedAt = timestamppb.New(issuedAt.Time)
		components[name] = component
	}
	defer rows.Close()

	return &pb_update.Update_Response{
		Components: components,
	}, nil
}
