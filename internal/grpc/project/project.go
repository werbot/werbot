package project

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/werbot/werbot/internal/crypto"
	projectpb "github.com/werbot/werbot/internal/grpc/project/proto"
	"github.com/werbot/werbot/internal/trace"
)

// ListProjects is ...
func (h *Handler) ListProjects(ctx context.Context, in *projectpb.ListProjects_Request) (*projectpb.ListProjects_Response, error) {
	response := &projectpb.ListProjects_Response{}

	sqlSearch := h.DB.SQLAddWhere(in.GetQuery())
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := h.DB.Conn.QueryContext(ctx, `
    SELECT
      "project"."id",
      "project"."owner_id",
      "project"."title",
      "project"."login",
      "project"."created_at",
      (
        SELECT COUNT(*)
        FROM "project_member"
        WHERE "project_id" = "project"."id"
      ) AS "count_members",
      (
        SELECT COUNT(*)
        FROM "server"
        WHERE "project_id" = "project"."id"
      ) AS "count_servers"
    FROM
      "project"
      LEFT JOIN "project_api" ON "project"."id" = "project_api"."project_id"
  `+sqlSearch+sqlFooter)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	for rows.Next() {
		var countMembers, countServers int32
		var createdAt pgtype.Timestamp
		project := &projectpb.Project_Response{}
		err = rows.Scan(&project.ProjectId,
			&project.OwnerId,
			&project.Title,
			&project.Login,
			&createdAt,
			&countMembers,
			&countServers,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		project.CreatedAt = timestamppb.New(createdAt.Time)
		project.MembersCount = countMembers
		project.ServersCount = countServers
		response.Projects = append(response.Projects, project)
	}
	defer rows.Close()

	// Total count for pagination
	err = h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM
      "project"
      LEFT JOIN "project_api" ON "project"."id" = "project_api"."project_id"
  `+sqlSearch,
	).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		return nil, trace.Error(err, log, nil)
	}

	return response, nil
}

// Project is ...
func (h *Handler) Project(ctx context.Context, in *projectpb.Project_Request) (*projectpb.Project_Response, error) {
	var countMembers, countServers int32
	var createdAt pgtype.Timestamp
	response := &projectpb.Project_Response{}

	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "project"."title",
      "project"."login",
      "project"."created_at",
      (
        SELECT COUNT(*)
        FROM "project_member"
        WHERE "project_id" = "project"."id"
      ) AS "count_members",
      (
        SELECT COUNT(*)
        FROM "server"
        WHERE "project_id" = "project"."id"
      ) AS "count_servers"
    FROM
      "project"
      LEFT JOIN "project_api" ON "project"."id" = "project_api"."project_id"
    WHERE
      "project"."owner_id" = $1
      AND "project"."id" = $2
  `, in.GetOwnerId(), in.GetProjectId(),
	).Scan(&response.Title,
		&response.Login,
		&createdAt,
		&countMembers,
		&countServers,
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	response.CreatedAt = timestamppb.New(createdAt.Time)
	response.MembersCount = countMembers
	response.ServersCount = countServers
	return response, nil
}

// AddProject is ...
func (h *Handler) AddProject(ctx context.Context, in *projectpb.AddProject_Request) (*projectpb.AddProject_Response, error) {
	response := &projectpb.AddProject_Response{}

	tx, err := h.DB.Conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCreateError)
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
    INSERT INTO "project" ("owner_id", "title", "login")
    VALUES ($1, $2, $3)
    RETURNING "id"
  `, in.GetOwnerId(), in.GetTitle(), in.GetLogin(),
	).Scan(&response.ProjectId)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
	}

	_, err = tx.ExecContext(ctx, `
    INSERT INTO "public"."project_api" ("project_id", "api_key", "api_secret", "online")
    VALUES ($1, $2, $3, 't')
  `, response.ProjectId, crypto.NewPassword(37, false), crypto.NewPassword(37, false))
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
	}

	if err := tx.Commit(); err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCommitError)
	}

	return response, nil
}

// UpdateProject is ...
func (h *Handler) UpdateProject(ctx context.Context, in *projectpb.UpdateProject_Request) (*projectpb.UpdateProject_Response, error) {
	_, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "project"
    SET
      "title" = $1,
      "login" = $2
    WHERE
      "id" = $3
      AND "owner_id" = $4
  `,
		in.GetTitle(),
		in.GetLogin(),
		in.GetProjectId(),
		in.GetOwnerId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	return &projectpb.UpdateProject_Response{}, nil
}

// DeleteProject is ...
func (h *Handler) DeleteProject(ctx context.Context, in *projectpb.DeleteProject_Request) (*projectpb.DeleteProject_Response, error) {
	tx, err := h.DB.Conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCreateError)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
    DELETE FROM "project"
    WHERE
      "id" = $1
      AND "owner_id" = $2
  `, in.GetProjectId(), in.GetOwnerId())
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToDelete)
	}

	_, err = tx.ExecContext(ctx, `
    DELETE FROM "project_api"
    WHERE "id" = $1
  `, in.GetProjectId())
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToDelete)
	}

	if err := tx.Commit(); err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCommitError)
	}

	return &projectpb.DeleteProject_Response{}, nil
}

// Key is ...
func (h *Handler) Key(ctx context.Context, in *projectpb.Key_Request) (*projectpb.Key_Response, error) {
	response := &projectpb.Key_Response{}
	return response, nil
}

// AddKey is ...
func (h *Handler) AddKey(ctx context.Context, in *projectpb.AddKey_Request) (*projectpb.AddKey_Response, error) {
	response := &projectpb.AddKey_Response{}
	return response, nil
}

// UpdateKey is ...
func (h *Handler) UpdateKey(ctx context.Context, in *projectpb.UpdateKey_Request) (*projectpb.UpdateKey_Response, error) {
	response := &projectpb.UpdateKey_Response{}
	return response, nil
}

// DeleteKey is ...
func (h *Handler) DeleteKey(ctx context.Context, in *projectpb.DeleteKey_Request) (*projectpb.DeleteKey_Response, error) {
	response := &projectpb.DeleteKey_Response{}
	return response, nil
}
