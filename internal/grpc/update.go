package grpc

import (
	"context"
	"errors"

	"github.com/jackc/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb_update "github.com/werbot/werbot/internal/grpc/proto/update"
)

type update struct {
	pb_update.UnimplementedUpdateHandlersServer
}

// GetUpdate is ...
func (u *update) GetUpdate(ctx context.Context, in *pb_update.GetUpdate_Request) (*pb_update.GetUpdate_Response, error) {
	rows, err := db.Conn.Query(`SELECT 
			"component", 
			"description", 
			"version", 
			"version_after", 
			"issued_at" 
		FROM 
			"update"`)
	if err != nil {
		return nil, errors.New("GetUpdate failed")
	}

	components := map[string]*pb_update.GetUpdate_Response_Component{}

	for rows.Next() {
		component := pb_update.GetUpdate_Response_Component{}
		var name string
		var issuedAt pgtype.Timestamp

		err = rows.Scan(
			&name,
			&component.Description,
			&component.Version,
			&component.VersionAfter,
			&issuedAt,
		)
		if err != nil {
			return nil, errors.New("GetUpdate Scan")
		}

		component.IssuedAt = timestamppb.New(issuedAt.Time)

		components[name] = &component
	}
	defer rows.Close()

	return &pb_update.GetUpdate_Response{
		Components: components,
	}, nil
}
