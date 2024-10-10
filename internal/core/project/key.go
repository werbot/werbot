package project

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
	projectpb "github.com/werbot/werbot/internal/core/project/proto/project"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProjectKeys is ...
func (h *Handler) ProjectKeys(ctx context.Context, in *projectpb.ProjectKeys_Request) (*projectpb.ProjectKeys_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &projectpb.ProjectKeys_Response{}

	// Total count for pagination
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM
      "project_api"
      INNER JOIN "project" ON "project_api"."project_id" = "project"."id"
    WHERE
      "project"."owner_id" = $1
      AND "project_api"."project_id" = $2
  `,
		in.GetOwnerId(),
		in.GetProjectId(),
	).Scan(&response.Total)
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
      "project_api"."id",
      "project_api"."api_key",
      "project_api"."api_secret",
      "project_api"."active",
      "project_api"."locked_at",
      "project_api"."archived_at",
      "project_api"."updated_at",
      "project_api"."created_at"
    FROM
      "project_api"
      INNER JOIN "project" ON "project_api"."project_id" = "project"."id"
    WHERE
      "project"."owner_id" = $1
      AND "project_api"."project_id" = $2
  `, sqlFooter)
	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery, in.GetOwnerId(), in.GetProjectId())
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		var lockedAt, archivedAt, updatedAt, createdAt pgtype.Timestamp
		key := &projectpb.ProjectKey_Response{}
		err = rows.Scan(
			&key.KeyId,
			&key.Key,
			&key.Secret,
			&key.Online,
			&lockedAt,
			&archivedAt,
			&updatedAt,
			&createdAt,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		protoutils.SetPgtypeTimestamps(key, map[string]pgtype.Timestamp{
			"locked_at":   lockedAt,
			"archived_at": archivedAt,
			"updated_at":  updatedAt,
			"created_at":  createdAt,
		})

		if !in.GetIsAdmin() {
			ghoster.Secrets(key, true)
		}

		response.Keys = append(response.Keys, key)
	}

	return response, nil
}

// ProjectKey is ...
func (h *Handler) ProjectKey(ctx context.Context, in *projectpb.ProjectKey_Request) (*projectpb.ProjectKey_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &projectpb.ProjectKey_Response{}

	switch in.GetType().(type) {
	case *projectpb.ProjectKey_Request_Public:
		err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT "project_id"
      FROM "project_api"
      WHERE "api_key" = $1 AND "active" = TRUE
    `, in.GetPublic().GetKey()).Scan(&response.ProjectId)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

	case *projectpb.ProjectKey_Request_Private:
		var lockedAt, archivedAt, updatedAt, createdAt pgtype.Timestamp
		err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT
        "project_api"."api_key",
        "project_api"."api_secret",
        "project_api"."active",
        "project_api"."locked_at",
        "project_api"."archived_at",
        "project_api"."updated_at",
        "project_api"."created_at"
      FROM
        "project_api"
        INNER JOIN "project" ON "project_api"."project_id" = "project"."id"
      WHERE
        "project"."owner_id" = $1
        AND "project_api"."project_id" = $2
        AND "project_api"."id" = $3
    `,
			in.GetPrivate().GetOwnerId(),
			in.GetPrivate().GetProjectId(),
			in.GetPrivate().GetKeyId(),
		).Scan(
			&response.Key,
			&response.Secret,
			&response.Online,
			&lockedAt,
			&archivedAt,
			&updatedAt,
			&createdAt,
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

		if !in.GetPrivate().GetIsAdmin() {
			ghoster.Secrets(response, true)
		}
	}

	return response, nil
}

// AddProjectKey is ...
func (h *Handler) AddProjectKey(ctx context.Context, in *projectpb.AddProjectKey_Request) (*projectpb.AddProjectKey_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &projectpb.AddProjectKey_Response{
		Key: crypto.NewPassword(37, false),
	}

	err := h.DB.Conn.QueryRowContext(ctx, `
    INSERT INTO "project_api" ("project_id", "api_key", "api_secret", "active")
    SELECT $2, $3, $4, TRUE
    WHERE EXISTS (
      SELECT 1
      FROM "project"
      WHERE "owner_id" = $1 AND "id" = $2
    )
    RETURNING "id"
  `,
		in.GetOwnerId(),
		in.GetProjectId(),
		response.GetKey(),
		crypto.NewPassword(37, false),
	).Scan(&response.KeyId)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
	}

	return response, nil
}

// DeleteProjectKey is ...
func (h *Handler) DeleteProjectKey(ctx context.Context, in *projectpb.DeleteProjectKey_Request) (*projectpb.DeleteProjectKey_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	result, err := h.DB.Conn.ExecContext(ctx, `
    DELETE FROM "project_api"
    USING "project"
    WHERE "project_api"."project_id" = "project"."id"
      AND "project"."owner_id" = $1
      AND "project_api"."project_id" = $2
      AND "project_api"."id" = $3
  `,
		in.GetOwnerId(),
		in.GetProjectId(),
		in.GetKeyId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToDelete)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgMemberNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &projectpb.DeleteProjectKey_Response{}, nil
}
