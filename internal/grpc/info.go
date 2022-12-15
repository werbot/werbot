package grpc

import (
	"context"

	pb_info "github.com/werbot/werbot/api/proto/info"
	pb_user "github.com/werbot/werbot/api/proto/user"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
)

type info struct {
	pb_info.UnimplementedInfoHandlersServer
}

// GetInfo is ...
func (i *info) GetInfo(ctx context.Context, in *pb_info.GetInfo_Request) (*pb_info.GetInfo_Response, error) {
	var users, projects, servers int32

	sqlProjects, _ := sanitize.SQL(` WHERE "owner_id" = $1`, in.GetUserId())
	sqlServers, _ := sanitize.SQL(` INNER JOIN "project" ON "server"."project_id" = "project"."id" WHERE "project"."owner_id" = $1`, in.GetUserId())

	if in.Role == pb_user.RoleUser_ADMIN {
		db.Conn.QueryRowx(`SELECT COUNT(*) AS users FROM "user"`).Scan(&users)
		sqlProjects = ""
		sqlServers = ""
	}

	db.Conn.QueryRow(`SELECT COUNT(*) AS projects FROM "project"` + sqlProjects).Scan(&projects)
	db.Conn.QueryRow(`SELECT COUNT(*) AS servers FROM "server"` + sqlServers).Scan(&servers)

	return &pb_info.GetInfo_Response{
		Users:    users,
		Projects: projects,
		Servers:  servers,
	}, nil
}
