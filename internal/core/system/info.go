package system

import (
	"context"

	systempb "github.com/werbot/werbot/internal/core/system/proto/system"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProfileMetrics is ...
func (h *Handler) ProfileMetrics(ctx context.Context, in *systempb.ProfileMetrics_Request) (*systempb.ProfileMetrics_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &systempb.ProfileMetrics_Response{}
	var err error

	if in.GetIsAdmin() && in.GetProfileId() == "00000000-0000-0000-0000-000000000000" {
		query := `
      SELECT
        (SELECT COUNT(*) FROM "profile"),
        (SELECT COUNT(*) FROM "project"),
        (SELECT COUNT(*)
          FROM "scheme"
          INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
        )
    `
		err = h.DB.Conn.QueryRowContext(ctx, query).Scan(&response.Profiles, &response.Projects, &response.Schemes)
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
		err = h.DB.Conn.QueryRowContext(ctx, query, in.GetProfileId()).Scan(&response.Projects, &response.Schemes)
	}

	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	return response, nil
}
