package grpc

import (
	"context"
	"database/sql"

	"github.com/jackc/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/werbot/werbot/internal/crypto"

	pb_project "github.com/werbot/werbot/api/proto/project"
)

type project struct {
	pb_project.UnimplementedProjectHandlersServer
}

// ListProjects is ...
func (p *project) ListProjects(ctx context.Context, in *pb_project.ListProjects_Request) (*pb_project.ListProjects_Response, error) {
	sqlSearch := service.db.SQLAddWhere(in.GetQuery())
	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := service.db.Conn.Query(`SELECT
			"project"."id",
			"project"."owner_id",
			"project"."title",
			"project"."login",
			"project"."created",
			( SELECT COUNT ( * ) FROM "project_member" WHERE "project_id" = "project"."id" ) AS "count_members",
			( SELECT COUNT ( * ) FROM "server" WHERE "project_id" = "project"."id" ) AS "count_servers"
		FROM
			"project"
			LEFT JOIN "project_api" ON "project"."id" = "project_api"."project_id"` + sqlSearch + sqlFooter)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToSelect
	}

	projects := []*pb_project.Project_Response{}
	for rows.Next() {
		var countMembers, countServers int32
		var created pgtype.Timestamp
		project := new(pb_project.Project_Response)
		err = rows.Scan(&project.ProjectId,
			&project.OwnerId,
			&project.Title,
			&project.Login,
			&created,
			&countMembers,
			&countServers,
		)
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, errFailedToScan
		}
		project.Created = timestamppb.New(created.Time)
		project.MembersCount = countMembers
		project.ServersCount = countServers
		projects = append(projects, project)
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	err = service.db.Conn.QueryRow(`SELECT
			COUNT (*)
		FROM
			"project"
			LEFT JOIN "project_api" ON "project"."id" = "project_api"."project_id"` + sqlSearch).
		Scan(&total)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToScan
	}

	return &pb_project.ListProjects_Response{
		Total:    total,
		Projects: projects,
	}, nil
}

// Project is ...
func (p *project) Project(ctx context.Context, in *pb_project.Project_Request) (*pb_project.Project_Response, error) {
	var countMembers, countServers int32
	var created pgtype.Timestamp
	project := new(pb_project.Project_Response)
	err := service.db.Conn.QueryRow(`SELECT
			"project"."title",
			"project"."login",
			"project"."created",
			( SELECT COUNT ( * ) FROM "project_member" WHERE "project_id" = "project"."id" ) AS "count_members",
			( SELECT COUNT ( * ) FROM "server" WHERE "project_id" = "project"."id" ) AS "count_servers"
		FROM
			"project"
			LEFT JOIN "project_api" ON "project"."id" = "project_api"."project_id"
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
		service.log.ErrorGRPC(err)
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		return nil, errFailedToScan
	}

	project.Created = timestamppb.New(created.Time)
	project.MembersCount = countMembers
	project.ServersCount = countServers
	return project, nil
}

// CreateProject is ...
func (p *project) CreateProject(ctx context.Context, in *pb_project.CreateProject_Request) (*pb_project.CreateProject_Response, error) {
	//	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
	//		return nil, errNotFound
	//	}

	tx, err := service.db.Conn.Beginx()
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errTransactionCreateError
	}

	var id string
	err = tx.QueryRow(`INSERT
    INTO "project" (
      "owner_id",
      "title",
      "login",
      "created"
    )
    VALUES ($1, $2, $3, NOW())
    RETURNING "id"`,
		in.GetOwnerId(),
		in.GetTitle(),
		in.GetLogin(),
	).Scan(&id)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToAdd
	}

	data, err := tx.Exec(`INSERT
    INTO "public"."project_api" (
      "project_id",
      "api_key",
      "api_secret",
      "online",
      "created"
    )
    VALUES ($1, $2, $3, 't', NOW())`,
		id,
		crypto.NewPassword(37, false),
		crypto.NewPassword(37, false),
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToAdd
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	if err = tx.Commit(); err != nil {
		service.log.ErrorGRPC(err)
		return nil, errTransactionCommitError
	}

	return &pb_project.CreateProject_Response{
		ProjectId: id,
	}, nil
}

// UpdateProject is ...
func (p *project) UpdateProject(ctx context.Context, in *pb_project.UpdateProject_Request) (*pb_project.UpdateProject_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	data, err := service.db.Conn.Exec(`UPDATE
        "project"
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
		service.log.ErrorGRPC(err)
		return nil, err
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_project.UpdateProject_Response{}, nil
}

// DeleteProject is ...
func (p *project) DeleteProject(ctx context.Context, in *pb_project.DeleteProject_Request) (*pb_project.DeleteProject_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	tx, err := service.db.Conn.Beginx()
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errTransactionCreateError
	}

	data, err := tx.Exec(`DELETE
		FROM
			"project"
		WHERE
			"id" = $1
			AND "owner_id" = $2`,
		in.GetProjectId(),
		in.GetOwnerId(),
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	data, err = tx.Exec(`DELETE
		FROM
			"project_api"
		WHERE
			"id" = $1`,
		in.GetProjectId(),
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, err
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	if err = tx.Commit(); err != nil {
		service.log.ErrorGRPC(err)
		return nil, errTransactionCommitError
	}

	return &pb_project.DeleteProject_Response{}, nil
}
