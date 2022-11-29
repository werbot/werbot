package grpc

import (
	"context"
	"errors"

	"github.com/jackc/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/werbot/werbot/internal/crypto"
	"github.com/werbot/werbot/internal/message"

	pb_project "github.com/werbot/werbot/internal/grpc/proto/project"
)

type project struct {
	pb_project.UnimplementedProjectHandlersServer
}

// ListProjects is ...
func (p *project) ListProjects(ctx context.Context, in *pb_project.ListProjects_Request) (*pb_project.ListProjects_Response, error) {
	sqlSearch := db.SQLAddWhere(in.GetQuery())
	sqlFooter := db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())

	rows, err := db.Conn.Query(`SELECT
			"project"."id",
			"project"."owner_id",
			"project"."title",
			"project"."login",
			"project"."created",
			( SELECT COUNT ( * ) FROM "project_member" WHERE "project_id" = "project"."id" ) AS "count_members",
			( SELECT COUNT ( * ) FROM "server" WHERE "project_id" = "project"."id" ) AS "count_servers" 
		FROM
			"project"
			LEFT JOIN "project_key" ON "project"."id" = "project_key"."project_id"` + sqlSearch + sqlFooter)
	if err != nil {
		return nil, err
	}

	projects := []*pb_project.GetProject_Response{}
	for rows.Next() {
		project := pb_project.GetProject_Response{}
		var created pgtype.Timestamp
		var countMembers, countServers int32

		err = rows.Scan(&project.ProjectId,
			&project.OwnerId,
			&project.Title,
			&project.Login,
			&created,
			&countMembers,
			&countServers,
		)
		if err != nil {
			return nil, err
		}

		project.Created = timestamppb.New(created.Time)
		project.MembersCount = countMembers
		project.ServersCount = countServers

		projects = append(projects, &project)
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	db.Conn.QueryRow(`SELECT COUNT (*) FROM "project"
			LEFT JOIN "project_key" ON "project"."id" = "project_key"."project_id"` + sqlSearch).Scan(&total)

	return &pb_project.ListProjects_Response{
		Total:    total,
		Projects: projects,
	}, nil
}

// GetProject is ...
func (p *project) GetProject(ctx context.Context, in *pb_project.GetProject_Request) (*pb_project.GetProject_Response, error) {
	project := pb_project.GetProject_Response{}
	var created pgtype.Timestamp
	var countMembers, countServers int32

	err := db.Conn.QueryRow(`SELECT
			"project"."title",
			"project"."login",
			"project"."created",
			( SELECT COUNT ( * ) FROM "project_member" WHERE "project_id" = "project"."id" ) AS "count_members",
			( SELECT COUNT ( * ) FROM "server" WHERE "project_id" = "project"."id" ) AS "count_servers" 
		FROM
			"project"
			LEFT JOIN "project_key" ON "project"."id" = "project_key"."project_id"
		WHERE
	    	"project"."owner_id" = $1 AND "project"."id" = $2`, in.GetOwnerId(), in.GetProjectId()).
		Scan(
			&project.Title,
			&project.Login,
			&created,
			&countMembers,
			&countServers,
		)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, errors.New(message.ErrNotFound)
		}
		return nil, err
	}

	project.Created = timestamppb.New(created.Time)
	project.MembersCount = countMembers
	project.ServersCount = countServers

	return &project, nil
}

// CreateProject is ...
func (p *project) CreateProject(ctx context.Context, in *pb_project.CreateProject_Request) (*pb_project.CreateProject_Response, error) {
	var id string
	err := db.Conn.QueryRow(`INSERT INTO "project" ( "owner_id", "title", "login", "created" ) VALUES ( $1, $2, $3, NOW( ) ) RETURNING "id"`,
		in.GetOwnerId(),
		in.GetTitle(),
		in.GetLogin(),
	).Scan(&id)
	if err != nil {
		return nil, err
	}

	_, err = db.Conn.Exec(`INSERT INTO "public"."project_key" ("project_id", "api_key", "api_secret", "online", "created") VALUES ($1, $2, $3, 't', NOW())`,
		id,
		crypto.NewPassword(37, false),
		crypto.NewPassword(37, false),
	)
	if err != nil {
		return nil, err
	}

	return &pb_project.CreateProject_Response{
		ProjectId: id,
	}, nil
}

// UpdateProject is ...
func (p *project) UpdateProject(ctx context.Context, in *pb_project.UpdateProject_Request) (*pb_project.UpdateProject_Response, error) {
	_, err := db.Conn.Exec(`UPDATE "project" 
			SET 
				"title" = $1
			WHERE
				"id" = $2 
				AND "owner_id" = $3`,
		in.GetTitle(),
		in.GetProjectId(),
		in.GetOwnerId(),
	)
	if err != nil {
		return nil, err
	}

	return &pb_project.UpdateProject_Response{}, nil
}

// DeleteProject is ...
func (p *project) DeleteProject(ctx context.Context, in *pb_project.DeleteProject_Request) (*pb_project.DeleteProject_Response, error) {
	if _, err := db.Conn.Exec(`DELETE 
		FROM 
			"project" 
		WHERE 
			"id" = $1 
			AND "owner_id" = $2`,
		in.GetProjectId(),
		in.GetOwnerId(),
	); err != nil {
		return &pb_project.DeleteProject_Response{}, err
	}

	if _, err := db.Conn.Exec(`DELETE 
		FROM 
			"project_key" 
		WHERE 
			"id" = $1`,
		in.GetProjectId(),
	); err != nil {
		return &pb_project.DeleteProject_Response{}, err
	}

	return &pb_project.DeleteProject_Response{}, nil
}
