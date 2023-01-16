package grpc

import (
	"context"

	infopb "github.com/werbot/werbot/api/proto/info"
	userpb "github.com/werbot/werbot/api/proto/user"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
)

type info struct {
	infopb.UnimplementedInfoHandlersServer
}

// UserMetrics is ...
func (i *info) UserMetrics(ctx context.Context, in *infopb.UserMetrics_Request) (*infopb.UserMetrics_Response, error) {
	response := new(infopb.UserMetrics_Response)

	sqlProjects, _ := sanitize.SQL(`WHERE "owner_id" = $1`,
		in.GetUserId(),
	)
	sqlServers, _ := sanitize.SQL(`INNER JOIN "project" ON "server"."project_id" = "project"."id" WHERE "project"."owner_id" = $1`,
		in.GetUserId(),
	)

	if in.Role == userpb.Role_admin {
		service.db.Conn.QueryRow(`SELECT COUNT(*) AS users FROM "user"`).Scan(&response.Users)
		sqlProjects = ""
		sqlServers = ""

	}
	service.db.Conn.QueryRow(`SELECT COUNT(*) AS projects FROM "project" ` + sqlProjects).Scan(&response.Projects)
	service.db.Conn.QueryRow(`SELECT COUNT(*) AS servers FROM "server" ` + sqlServers).Scan(&response.Servers)

	return response, nil
}
