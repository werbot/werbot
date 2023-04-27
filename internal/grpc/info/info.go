package info

import (
	"context"

	infopb "github.com/werbot/werbot/internal/grpc/info/proto"
	userpb "github.com/werbot/werbot/internal/grpc/user/proto"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/trace"
)

// UserMetrics is ...
func (h *Handler) UserMetrics(ctx context.Context, in *infopb.UserMetrics_Request) (*infopb.UserMetrics_Response, error) {
	response := new(infopb.UserMetrics_Response)

	sqlProjects, err := sanitize.SQL(`WHERE "owner_id" = $1`, in.GetUserId())
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	sqlServers, err := sanitize.SQL(`INNER JOIN "project" ON "server"."project_id" = "project"."id" WHERE "project"."owner_id" = $1`, in.GetUserId())
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	if in.Role == userpb.Role_admin {
		err := h.DB.Conn.QueryRowContext(ctx, `SELECT COUNT(*) AS users FROM "user"`).Scan(&response.Users)
		if err != nil {
			return nil, trace.ErrorDB(err, h.Log)
		}
		sqlProjects = ""
		sqlServers = ""
	}
	err = h.DB.Conn.QueryRowContext(ctx, `SELECT COUNT(*) AS projects FROM "project" `+sqlProjects).Scan(&response.Projects)
	if err != nil {
		return nil, trace.ErrorDB(err, h.Log)
	}

	err = h.DB.Conn.QueryRowContext(ctx, `SELECT COUNT(*) AS servers FROM "server" `+sqlServers).Scan(&response.Servers)
	if err != nil {
		return nil, trace.ErrorDB(err, h.Log)
	}

	return response, nil
}
