package info

import (
	"context"

	infopb "github.com/werbot/werbot/api/proto/info"
	userpb "github.com/werbot/werbot/api/proto/user"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
)

// UserMetrics is ...
func (h *Handler) UserMetrics(ctx context.Context, in *infopb.UserMetrics_Request) (*infopb.UserMetrics_Response, error) {
	response := new(infopb.UserMetrics_Response)

	sqlProjects, _ := sanitize.SQL(`WHERE "owner_id" = $1`,
		in.GetUserId(),
	)
	sqlServers, _ := sanitize.SQL(`INNER JOIN "project" ON "server"."project_id" = "project"."id" WHERE "project"."owner_id" = $1`,
		in.GetUserId(),
	)

	if in.Role == userpb.Role_admin {
		h.DB.Conn.QueryRow(`SELECT COUNT(*) AS users FROM "user"`).Scan(&response.Users)
		sqlProjects = ""
		sqlServers = ""

	}
	h.DB.Conn.QueryRow(`SELECT COUNT(*) AS projects FROM "project" ` + sqlProjects).Scan(&response.Projects)
	h.DB.Conn.QueryRow(`SELECT COUNT(*) AS servers FROM "server" ` + sqlServers).Scan(&response.Servers)

	return response, nil
}
