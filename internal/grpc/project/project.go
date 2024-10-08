package project

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	projectpb "github.com/werbot/werbot/internal/grpc/project/proto/project"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
)

// Projects is ...
func (h *Handler) Projects(ctx context.Context, in *projectpb.Projects_Request) (*projectpb.Projects_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &projectpb.Projects_Response{}

	// Total count for pagination
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM "project"
    WHERE "owner_id" = $1
  `, in.GetOwnerId()).Scan(&response.Total)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, trace.Error(err, log, nil)
	}
	if response.Total == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgProjectNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// List records
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	baseQuery := postgres.SQLGluing(`
    SELECT
      "project"."id",
      "project"."owner_id",
      "project"."title",
      "project"."alias",
      "project"."locked_at",
      "project"."archived_at",
      "project"."updated_at",
      "project"."created_at",
      COUNT(DISTINCT "project_member"."id") AS member_count,
      COUNT(CASE WHEN "scheme"."scheme_type" = 100 THEN 1 END) AS scheme_type_100_count,
      COUNT(CASE WHEN "scheme"."scheme_type" = 200 THEN 1 END) AS scheme_type_200_count,
      COUNT(CASE WHEN "scheme"."scheme_type" = 300 THEN 1 END) AS scheme_type_300_count,
      COUNT(CASE WHEN "scheme"."scheme_type" = 400 THEN 1 END) AS scheme_type_400_count,
      COUNT(CASE WHEN "scheme"."scheme_type" = 500 THEN 1 END) AS scheme_type_500_count,
      COUNT(CASE WHEN "scheme"."scheme_type" = 600 THEN 1 END) AS scheme_type_600_count
    FROM
      "project"
    LEFT JOIN "project_member" ON "project_member"."project_id" = "project"."id"
    LEFT JOIN "scheme" ON "scheme"."project_id" = "project"."id"
    WHERE "project"."owner_id" = $1
    GROUP BY "project"."id"
`, sqlFooter)

	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery, in.GetOwnerId())
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	for rows.Next() {
		var lockedAt, archivedAt, updatedAt, createdAt pgtype.Timestamp
		project := &projectpb.Project_Response{}
		err = rows.Scan(
			&project.ProjectId,
			&project.OwnerId,
			&project.Title,
			&project.Alias,
			&lockedAt,
			&archivedAt,
			&updatedAt,
			&createdAt,
			&project.MembersCount,
			&project.ServersCount,
			&project.DatabasesCount,
			&project.ApplicationsCount,
			&project.DesktopsCount,
			&project.ContainersCount,
			&project.CloudsCount,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		protoutils.SetPgtypeTimestamps(project, map[string]pgtype.Timestamp{
			"locked_at":   lockedAt,
			"archived_at": archivedAt,
			"updated_at":  updatedAt,
			"created_at":  createdAt,
		})

		if !in.GetIsAdmin() {
			ghoster.Secrets(project, true)
		}

		response.Projects = append(response.Projects, project)
	}
	defer rows.Close()

	return response, nil
}

// Project is ...
func (h *Handler) Project(ctx context.Context, in *projectpb.Project_Request) (*projectpb.Project_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	var lockedAt, archivedAt, updatedAt, createdAt pgtype.Timestamp
	response := &projectpb.Project_Response{}

	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "project"."title",
      "project"."alias",
      "project"."locked_at",
      "project"."archived_at",
      "project"."updated_at",
      "project"."created_at",
      COUNT(DISTINCT "project_member"."id") AS member_count,
      COUNT(CASE WHEN "scheme"."scheme_type" = 100 THEN 1 END) AS scheme_type_100_count,
      COUNT(CASE WHEN "scheme"."scheme_type" = 200 THEN 1 END) AS scheme_type_200_count,
      COUNT(CASE WHEN "scheme"."scheme_type" = 300 THEN 1 END) AS scheme_type_300_count,
      COUNT(CASE WHEN "scheme"."scheme_type" = 400 THEN 1 END) AS scheme_type_400_count,
      COUNT(CASE WHEN "scheme"."scheme_type" = 500 THEN 1 END) AS scheme_type_500_count,
      COUNT(CASE WHEN "scheme"."scheme_type" = 600 THEN 1 END) AS scheme_type_600_count
    FROM
      "project"
    LEFT JOIN "project_member" ON "project_member"."project_id" = "project"."id"
    LEFT JOIN "scheme" ON "scheme"."project_id" = "project"."id"
    WHERE
      "project"."owner_id" = $1
      AND "project"."id" = $2
    GROUP BY "project"."id"
  `,
		in.GetOwnerId(),
		in.GetProjectId(),
	).Scan(
		&response.Title,
		&response.Alias,
		&lockedAt,
		&archivedAt,
		&updatedAt,
		&createdAt,
		&response.MembersCount,
		&response.ServersCount,
		&response.DatabasesCount,
		&response.ApplicationsCount,
		&response.DesktopsCount,
		&response.ContainersCount,
		&response.CloudsCount,
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	protoutils.SetPgtypeTimestamps(response, map[string]pgtype.Timestamp{
		"locked_at":   lockedAt,
		"archived_at": archivedAt,
		"updated_at":  updatedAt,
		"created_at":  createdAt,
	})

	if !in.GetIsAdmin() {
		ghoster.Secrets(response, true)
	}

	return response, nil
}

// AddProject is ...
func (h *Handler) AddProject(ctx context.Context, in *projectpb.AddProject_Request) (*projectpb.AddProject_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &projectpb.AddProject_Response{}

	tx, err := h.DB.Conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCreateError)
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
    INSERT INTO "project" ("owner_id", "title", "alias")
    SELECT $1, $2, $3
    WHERE EXISTS (
      SELECT 1
      FROM "user"
      WHERE "id" = $1
    )
    RETURNING "id"
  `,
		in.GetOwnerId(),
		in.GetTitle(),
		in.GetAlias(),
	).Scan(&response.ProjectId)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
	}

	_, err = tx.ExecContext(ctx, `
    INSERT INTO "project_api" ("project_id", "api_key", "api_secret", "active")
    VALUES ($1, $2, $3, TRUE)
  `, response.GetProjectId(), crypto.NewPassword(37, false), crypto.NewPassword(37, false))
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
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	var column string
	var value any

	switch in.GetSetting().(type) {
	case *projectpb.UpdateProject_Request_Alias:
		if !in.GetIsAdmin() {
			errGRPC := status.Error(codes.InvalidArgument, "setting: exactly one field is required in oneof")
			return nil, trace.Error(errGRPC, log, nil)
		}
		column = "alias"
		value = in.GetAlias()
	case *projectpb.UpdateProject_Request_Title:
		column = "title"
		value = in.GetTitle()
	}

	query := fmt.Sprintf(`
    UPDATE "project"
    SET "%s" = $1
    WHERE "id" = $2 AND "owner_id" = $3
  `, column)

	result, err := h.DB.Conn.ExecContext(ctx, query,
		value,
		in.GetProjectId(),
		in.GetOwnerId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgProjectNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &projectpb.UpdateProject_Response{}, nil
}

// DeleteProject is ...
func (h *Handler) DeleteProject(ctx context.Context, in *projectpb.DeleteProject_Request) (*projectpb.DeleteProject_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	archivedAt := time.Now().AddDate(0, 1, 0)

	result, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "project"
    SET
      "locked_at" = NOW(),
      "archived_at" = $3
    WHERE "id" = $1 AND "owner_id" = $2
  `,
		in.GetProjectId(),
		in.GetOwnerId(),
		archivedAt,
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToDelete)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgProjectNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}
	// TODO send project delete email

	return &projectpb.DeleteProject_Response{}, nil
}
