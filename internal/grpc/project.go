package grpc

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	projectpb "github.com/werbot/werbot/api/proto/project"
	"github.com/werbot/werbot/internal/crypto"
)

type project struct {
	projectpb.UnimplementedProjectHandlersServer
}

// ListProjects is ...
func (p *project) ListProjects(ctx context.Context, in *projectpb.ListProjects_Request) (*projectpb.ListProjects_Response, error) {
	response := new(projectpb.ListProjects_Response)

	sqlSearch := service.db.SQLAddWhere(in.GetQuery())
	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := service.db.Conn.Query(`SELECT
			"project"."id",
			"project"."owner_id",
			"project"."title",
			"project"."login",
			"project"."created",
			( SELECT COUNT (*) FROM "project_member" WHERE "project_id" = "project"."id" ) AS "count_members",
			( SELECT COUNT (*) FROM "server" WHERE "project_id" = "project"."id" ) AS "count_servers"
		FROM "project"
			LEFT JOIN "project_api" ON "project"."id" = "project_api"."project_id"` + sqlSearch + sqlFooter)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	for rows.Next() {
		var countMembers, countServers int32
		var created pgtype.Timestamp
		project := new(projectpb.Project_Response)
		err = rows.Scan(&project.ProjectId,
			&project.OwnerId,
			&project.Title,
			&project.Login,
			&created,
			&countMembers,
			&countServers,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			service.log.FromGRPC(err).Send()
			return nil, errServerError
		}
		project.Created = timestamppb.New(created.Time)
		project.MembersCount = countMembers
		project.ServersCount = countServers

		response.Projects = append(response.Projects, project)
	}
	defer rows.Close()

	// Total count for pagination
	err = service.db.Conn.QueryRow(`SELECT COUNT (*)
		FROM "project"
			LEFT JOIN "project_api" ON "project"."id" = "project_api"."project_id"` + sqlSearch,
	).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	return response, nil
}

// Project is ...
func (p *project) Project(ctx context.Context, in *projectpb.Project_Request) (*projectpb.Project_Response, error) {
	var countMembers, countServers int32
	var created pgtype.Timestamp
	response := new(projectpb.Project_Response)

	err := service.db.Conn.QueryRow(`SELECT
			"project"."title",
			"project"."login",
			"project"."created",
			( SELECT COUNT (*) FROM "project_member" WHERE "project_id" = "project"."id" ) AS "count_members",
			( SELECT COUNT (*) FROM "server" WHERE "project_id" = "project"."id" ) AS "count_servers"
		FROM "project"
			LEFT JOIN "project_api" ON "project"."id" = "project_api"."project_id"
		WHERE "project"."owner_id" = $1 AND "project"."id" = $2`,
		in.GetOwnerId(),
		in.GetProjectId(),
	).Scan(&response.Title,
		&response.Login,
		&created,
		&countMembers,
		&countServers,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	response.Created = timestamppb.New(created.Time)
	response.MembersCount = countMembers
	response.ServersCount = countServers

	return response, nil
}

// AddProject is ...
func (p *project) AddProject(ctx context.Context, in *projectpb.AddProject_Request) (*projectpb.AddProject_Response, error) {
	response := new(projectpb.AddProject_Response)

	tx, err := service.db.Conn.Begin()
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errTransactionCreateError
	}

	err = tx.QueryRow(`INSERT INTO "project" ("owner_id", "title", "login")
    VALUES ($1, $2, $3) RETURNING "id"`,
		in.GetOwnerId(),
		in.GetTitle(),
		in.GetLogin(),
	).Scan(&response.ProjectId)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	data, err := tx.Exec(`INSERT INTO "public"."project_api" ("project_id", "api_key", "api_secret", "online")
    VALUES ($1, $2, $3, 't')`,
		response.ProjectId,
		crypto.NewPassword(37, false),
		crypto.NewPassword(37, false),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	if err = tx.Commit(); err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errTransactionCommitError
	}

	return response, nil
}

// UpdateProject is ...
func (p *project) UpdateProject(ctx context.Context, in *projectpb.UpdateProject_Request) (*projectpb.UpdateProject_Response, error) {
	if !isOwnerProject(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	response := new(projectpb.UpdateProject_Response)

	data, err := service.db.Conn.Exec(`UPDATE "project" SET "title" = $1, "last_update" = NOW() WHERE "id" = $2 AND "owner_id" = $3`,
		in.GetTitle(),
		in.GetProjectId(),
		in.GetOwnerId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, err
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// DeleteProject is ...
func (p *project) DeleteProject(ctx context.Context, in *projectpb.DeleteProject_Request) (*projectpb.DeleteProject_Response, error) {
	if !isOwnerProject(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	response := new(projectpb.DeleteProject_Response)

	tx, err := service.db.Conn.Begin()
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errTransactionCreateError
	}

	data, err := tx.Exec(`DELETE FROM "project" WHERE "id" = $1 AND "owner_id" = $2`,
		in.GetProjectId(),
		in.GetOwnerId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	data, err = tx.Exec(`DELETE FROM "project_api" WHERE "id" = $1`,
		in.GetProjectId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, err
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	if err = tx.Commit(); err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errTransactionCommitError
	}

	return response, nil
}

// TODO Key is ...
func (p *project) Key(ctx context.Context, in *projectpb.Key_Request) (*projectpb.Key_Response, error) {
	response := new(projectpb.Key_Response)
	return response, nil
}

// TODO AddKey is ...
func (p *project) AddKey(ctx context.Context, in *projectpb.AddKey_Request) (*projectpb.AddKey_Response, error) {
	response := new(projectpb.AddKey_Response)
	return response, nil
}

// TODO UpdateKey is ...
func (p *project) UpdateKey(ctx context.Context, in *projectpb.UpdateKey_Request) (*projectpb.UpdateKey_Response, error) {
	response := new(projectpb.UpdateKey_Response)
	return response, nil
}

// TODO DeleteKey is ...
func (p *project) DeleteKey(ctx context.Context, in *projectpb.DeleteKey_Request) (*projectpb.DeleteKey_Response, error) {
	response := new(projectpb.DeleteKey_Response)
	return response, nil
}
