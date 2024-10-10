package system

import (
	"context"

	systempb "github.com/werbot/werbot/internal/core/system/proto/system"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UserMetrics is ...
func (h *Handler) UserMetrics(ctx context.Context, in *systempb.UserMetrics_Request) (*systempb.UserMetrics_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &systempb.UserMetrics_Response{}
	var err error

	if in.GetIsAdmin() && in.GetUserId() == "00000000-0000-0000-0000-000000000000" {
		query := `
      SELECT
        (SELECT COUNT(*) FROM "user"),
        (SELECT COUNT(*) FROM "project"),
        (SELECT COUNT(*)
          FROM "scheme"
          INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
        )
    `
		err = h.DB.Conn.QueryRowContext(ctx, query).Scan(&response.Users, &response.Projects, &response.Schemes)
	} else {
		query := `
      SELECT
        (SELECT COUNT(*) FROM "project" WHERE "owner_id" = $1),
        (SELECT COUNT(*)
          FROM "scheme"
          INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
          WHERE "project"."owner_id" = $1
        )
    `
		err = h.DB.Conn.QueryRowContext(ctx, query, in.GetUserId()).Scan(&response.Projects, &response.Schemes)
	}

	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	return response, nil
}
