package member

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	memberpb "github.com/werbot/werbot/internal/core/member/proto/member"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
)

// ProjectMembers is ...
func (h *Handler) ProjectMembers(ctx context.Context, in *memberpb.ProjectMembers_Request) (*memberpb.ProjectMembers_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &memberpb.ProjectMembers_Response{}
	sqlUserLimit := postgres.SQLColumnsNull(in.GetIsAdmin(), true, []string{`"project_member"."locked_at"`}) // if not admin

	// Total count for pagination
	baseQuery := postgres.SQLGluing(`
    SELECT COUNT(*)
    FROM
      "project_member"
      INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
    WHERE
      "project"."owner_id" = $1
      AND "project_member"."project_id" = $2
  `, sqlUserLimit)
	err := h.DB.Conn.QueryRowContext(ctx, baseQuery,
		in.GetOwnerId(),
		in.GetProjectId(),
	).Scan(&response.Total)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, trace.Error(err, log, nil)
	}
	if response.Total == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgMemberNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// List records
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	baseQuery = postgres.SQLGluing(`
    SELECT
      "project_member"."id",
      "project"."owner_id",
      "owner"."name",
      "project_member"."project_id",
      "project"."title",
      "project_member"."profile_id",
      "member"."name",
      "project_member"."role",
      "project_member"."active",
      "project_member"."online",
      "project_member"."updated_at",
      "project_member"."created_at",
      (
        SELECT COUNT(*)
        FROM "scheme_member"
        WHERE "project_member_id" = "project_member"."id"
      ) AS "count_schemes"
    FROM
      "project_member"
      INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
      INNER JOIN "profile" AS "member" ON "project_member"."profile_id" = "member"."id"
      INNER JOIN "profile" AS "owner" ON "project"."owner_id" = "owner"."id"
    WHERE
      "owner"."id" = $1
      AND "project"."id" = $2
  `, sqlUserLimit, sqlFooter)
	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery, in.GetOwnerId(), in.GetProjectId())
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		var updatedAt, createdAt pgtype.Timestamp
		member := &memberpb.ProjectMember_Response{}
		err = rows.Scan(
			&member.MemberId,
			&member.OwnerId,
			&member.OwnerName,
			&member.ProjectId,
			&member.ProjectName,
			&member.ProfileId,
			&member.Name,
			&member.Role,
			&member.Active,
			&member.Online,
			&updatedAt,
			&createdAt,
			&member.SchemesCount,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		protoutils.SetPgtypeTimestamps(member, map[string]pgtype.Timestamp{
			"updated_at": updatedAt,
			"created_at": createdAt,
		})

		if !in.GetIsAdmin() {
			ghoster.Secrets(member, true)
		}

		response.Members = append(response.Members, member)
	}

	return response, nil
}

// ProjectMember is ...
func (h *Handler) ProjectMember(ctx context.Context, in *memberpb.ProjectMember_Request) (*memberpb.ProjectMember_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	var updatedAt, createdAt pgtype.Timestamp
	response := &memberpb.ProjectMember_Response{
		MemberId:  in.GetMemberId(),
		OwnerId:   in.GetOwnerId(),
		ProjectId: in.GetProjectId(),
	}

	sqlUserLimit := postgres.SQLColumnsNull(in.GetIsAdmin(), true, []string{`"project_member"."locked_at"`}) // if not admin

	baseQuery := postgres.SQLGluing(`
    SELECT
      "owner"."name",
      "project"."title",
      "project_member"."profile_id",
      "member"."name",
      "project_member"."role",
      "project_member"."active",
      "project_member"."updated_at",
      "project_member"."created_at"
    FROM
      "project_member"
      INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
      INNER JOIN "profile" AS "member" ON "project_member"."profile_id" = "member"."id"
      INNER JOIN "profile" AS "owner" ON "project"."owner_id" = "owner"."id"
    WHERE
      "project_member"."id" = $1
      AND "project"."owner_id" = $2
      AND "project"."id" = $3
  `, sqlUserLimit)

	err := h.DB.Conn.QueryRowContext(ctx, baseQuery,
		in.GetMemberId(),
		in.GetOwnerId(),
		in.GetProjectId(),
	).Scan(
		&response.OwnerName,
		&response.ProjectName,
		&response.ProfileId,
		&response.Name,
		&response.Role,
		&response.Active,
		&updatedAt,
		&createdAt,
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	protoutils.SetPgtypeTimestamps(response, map[string]pgtype.Timestamp{
		"updated_at": updatedAt,
		"created_at": createdAt,
	})

	if !in.GetIsAdmin() {
		ghoster.Secrets(response, true)
	}

	return response, nil
}

// AddProjectMember is ...
func (h *Handler) AddProjectMember(ctx context.Context, in *memberpb.AddProjectMember_Request) (*memberpb.AddProjectMember_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &memberpb.AddProjectMember_Response{}
	err := h.DB.Conn.QueryRowContext(ctx, `
    INSERT INTO "project_member" ("project_id", "profile_id", "role", "active")
    SELECT $2, $3, $4, $5
    WHERE EXISTS (
      SELECT 1
      FROM "project"
      JOIN "profile" ON "profile"."id" = $3
      WHERE
        "project"."id" = $2
        AND "project"."owner_id" = $1
    )
    RETURNING "id"
  `,
		in.GetOwnerId(),
		in.GetProjectId(),
		in.GetProfileId(),
		in.GetRole(),
		in.GetActive(),
	).Scan(&response.MemberId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errGRPC := status.Error(codes.NotFound, trace.MsgNotFound)
			return nil, trace.Error(errGRPC, log, nil)
		}
		return nil, trace.Error(err, log, "")
	}

	return response, nil
}

// UpdateProjectMember is ...
func (h *Handler) UpdateProjectMember(ctx context.Context, in *memberpb.UpdateProjectMember_Request) (*memberpb.UpdateProjectMember_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	var column string
	var value any

	switch in.GetSetting().(type) {
	case *memberpb.UpdateProjectMember_Request_Role:
		if !in.GetIsAdmin() {
			errGRPC := status.Error(codes.InvalidArgument, "setting: exactly one field is required in oneof")
			return nil, trace.Error(errGRPC, log, nil)
		}
		column = "role"
		value = in.GetRole()
	case *memberpb.UpdateProjectMember_Request_Active:
		column = "active"
		value = in.GetActive()
	}

	query := fmt.Sprintf(`
    UPDATE "project_member"
    SET "%s" = $1
    FROM "project"
    WHERE
      "project_member"."project_id" = "project"."id"
      AND "project"."owner_id" = $2
      AND "project"."id" = $3
      AND "project_member"."id" = $4
  `, column)

	result, err := h.DB.Conn.ExecContext(ctx, query,
		value,
		in.GetOwnerId(),
		in.GetProjectId(),
		in.GetMemberId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgMemberNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &memberpb.UpdateProjectMember_Response{}, nil
}

// DeleteProjectMember is ...
func (h *Handler) DeleteProjectMember(ctx context.Context, in *memberpb.DeleteProjectMember_Request) (*memberpb.DeleteProjectMember_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	result, err := h.DB.Conn.ExecContext(ctx, `
    DELETE FROM "project_member" USING "project"
    WHERE
      "project_member"."project_id" = "project"."id"
      AND "project"."owner_id" = $1
      AND "project"."id" = $2
      AND "project_member"."id" = $3
  `,
		in.GetOwnerId(),
		in.GetProjectId(),
		in.GetMemberId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToDelete)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgSchemeNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &memberpb.DeleteProjectMember_Response{}, nil
}

// ProfilesWithoutProject is ...
// only foe admin
func (h *Handler) ProfilesWithoutProject(ctx context.Context, in *memberpb.ProfilesWithoutProject_Request) (*memberpb.ProfilesWithoutProject_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &memberpb.ProfilesWithoutProject_Response{}

	// Total count fo
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM
      "profile"
      LEFT JOIN "project_member" ON "profile"."id" = "project_member"."profile_id" AND "project_member"."project_id" = $1
    WHERE
      "project_member"."profile_id" IS NULL
      AND LOWER("profile"."alias") LIKE LOWER('%' || $2 || '%')
  `,
		in.GetProjectId(),
		in.GetAlias(),
	).Scan(&response.Total)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, trace.Error(err, log, nil)
	}
	if response.Total == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgMemberNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// List records
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	baseQuery := postgres.SQLGluing(`
    SELECT
      "profile"."id",
      "profile"."alias",
      "profile"."email"
    FROM
      "profile"
      LEFT JOIN "project_member" ON "profile"."id" = "project_member"."profile_id" AND "project_member"."project_id" = $1
    WHERE
      "project_member"."profile_id" IS NULL
      AND LOWER("profile"."alias") LIKE LOWER('%' || $2 || '%')
  `, sqlFooter)
	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery,
		in.GetProjectId(),
		in.GetAlias(),
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		user := &memberpb.ProfilesWithoutProject_Profile{}
		if err = rows.Scan(&user.ProfileId, &user.Alias, &user.Email); err != nil {
			return nil, trace.Error(err, log, nil)
		}
		response.Profiles = append(response.Profiles, user)
	}

	return response, nil
}
