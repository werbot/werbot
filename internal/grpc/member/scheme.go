package member

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	memberpb "github.com/werbot/werbot/internal/grpc/member/proto/member"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
)

// SchemeMembers is ...
func (h *Handler) SchemeMembers(ctx context.Context, in *memberpb.SchemeMembers_Request) (*memberpb.SchemeMembers_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &memberpb.SchemeMembers_Response{}

	// Total members
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM
      "scheme_member"
      INNER JOIN "project_member" ON "scheme_member"."project_member_id" = "project_member"."id"
      INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
      INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
    WHERE
      "project"."owner_id" = $1
      AND "scheme_member"."scheme_id" = $2
  `,
		in.GetOwnerId(),
		in.GetSchemeId(),
	).Scan(&response.Total)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, trace.Error(err, log, nil)
	}
	if response.Total == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgProjectNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// List records
	// sqlHook := sqlHookNoAdmin(in.IsAdmin)
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	baseQuery := postgres.SQLGluing(`
    SELECT
      "user"."id",
      "user"."alias",
      "user"."name",
      "user"."surname",
      "user"."email",
      "scheme_member"."id",
      "scheme_member"."active",
      "scheme_member"."online",
      "scheme_member"."locked_at",
      "scheme_member"."archived_at",
      "scheme_member"."updated_at",
      "scheme_member"."created_at"
    FROM
      "scheme_member"
      INNER JOIN "project_member" ON "scheme_member"."project_member_id" = "project_member"."id"
      INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
      INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
    WHERE
      "project"."owner_id" = $1
      AND "scheme_member"."scheme_id" = $2
  `, sqlFooter)
	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery, in.GetOwnerId(), in.GetSchemeId())
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		var lockedAt, archivedAt, updatedAt, createdAt pgtype.Timestamp
		member := &memberpb.SchemeMember_Response{}
		err = rows.Scan(
			&member.UserId,
			&member.UserAlias,
			&member.UserName,
			&member.UserSurname,
			&member.Email,
			&member.SchemeMemberId,
			&member.Active,
			&member.Online,
			&lockedAt,
			&archivedAt,
			&updatedAt,
			&createdAt,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		protoutils.SetPgtypeTimestamps(member, map[string]pgtype.Timestamp{
			"locked_at":   lockedAt,
			"archived_at": archivedAt,
			"updated_at":  updatedAt,
			"created_at":  createdAt,
		})

		if !in.GetIsAdmin() {
			ghoster.Secrets(member, true)
		}

		response.Members = append(response.Members, member)
	}

	return response, nil
}

// SchemeMember is ...
func (h *Handler) SchemeMember(ctx context.Context, in *memberpb.SchemeMember_Request) (*memberpb.SchemeMember_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	var lockedAt, archivedAt, updatedAt, createdAt pgtype.Timestamp
	response := &memberpb.SchemeMember_Response{
		// MemberId: in.GetMemberId(),
	}

	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "user"."id",
      "user"."alias",
      "user"."name",
      "user"."surname",
      "user"."email",
      "scheme_member"."id",
      "scheme_member"."active",
      "scheme_member"."online",
      "scheme_member"."locked_at",
      "scheme_member"."archived_at",
      "scheme_member"."updated_at",
      "scheme_member"."created_at"
    FROM
      "scheme_member"
      INNER JOIN "project_member" ON "scheme_member"."project_member_id" = "project_member"."id"
      INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
      INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
    WHERE
      "project"."owner_id" = $1
      AND "scheme_member"."scheme_id" = $2
      AND "scheme_member"."id" = $3
  `,
		in.GetOwnerId(),
		in.GetSchemeId(),
		in.GetSchemeMemberId(),
	).Scan(
		&response.UserId,
		&response.UserAlias,
		&response.UserName,
		&response.UserSurname,
		&response.Email,
		&response.SchemeMemberId,
		&response.Active,
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

	if !in.GetIsAdmin() {
		ghoster.Secrets(response, true)
	}

	return response, nil
}

// AddSchemeMember is ...
func (h *Handler) AddSchemeMember(ctx context.Context, in *memberpb.AddSchemeMember_Request) (*memberpb.AddSchemeMember_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &memberpb.AddSchemeMember_Response{}

	err := h.DB.Conn.QueryRowContext(ctx, `
    WITH project_check AS (
      SELECT 1
      FROM "scheme"
      INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
      WHERE "scheme"."id" = $1
        AND "project"."owner_id" = $4
    ),
    member_check AS (
      SELECT 1
      FROM "project_member"
      WHERE "project_member"."id" = $2
        AND "project_member"."project_id" IN (
          SELECT "project"."id"
          FROM "project"
          WHERE "project"."owner_id" = $4
        )
    ),
    existing_check AS (
      SELECT 1
      FROM "scheme_member"
      WHERE "scheme_member"."scheme_id" = $1
        AND "scheme_member"."project_member_id" = $2
    )

    INSERT INTO "scheme_member" ("scheme_id", "project_member_id", "active")
    SELECT $1, $2, $3
    WHERE EXISTS (SELECT 1 FROM project_check)
      AND EXISTS (SELECT 1 FROM member_check)
      AND NOT EXISTS (SELECT 1 FROM existing_check)
    RETURNING "id"
  `,
		in.GetSchemeId(),
		in.GetMemberId(),
		in.GetActive(),
		in.GetOwnerId(),
	).Scan(&response.SchemeMemberId)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
	}

	return response, nil
}

// UpdateSchemeMember is ...
func (h *Handler) UpdateSchemeMember(ctx context.Context, in *memberpb.UpdateSchemeMember_Request) (*memberpb.UpdateSchemeMember_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	var column string
	var value any

	switch in.GetSetting().(type) {
	case *memberpb.UpdateSchemeMember_Request_Active:
		column = "active"
		value = in.GetActive()
	case *memberpb.UpdateSchemeMember_Request_Online:
		column = "online"
		value = in.GetOnline()
	default:
		errGRPC := status.Error(codes.InvalidArgument, trace.MsgSettingNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	query := fmt.Sprintf(`
    UPDATE "scheme_member"
    SET "%s" = $1
    FROM
      "project_member"
      INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
    WHERE
      "scheme_member"."project_member_id" = "project_member"."id"
      AND "project"."owner_id" = $2
      AND "scheme_member"."scheme_id" = $3
      AND "scheme_member"."id" = $4
  `, column)

	result, err := h.DB.Conn.ExecContext(ctx, query,
		value,
		in.GetOwnerId(),
		in.GetSchemeId(),
		in.GetSchemeMemberId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgMemberNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &memberpb.UpdateSchemeMember_Response{}, nil
}

// DeleteSchemeMember is ...
func (h *Handler) DeleteSchemeMember(ctx context.Context, in *memberpb.DeleteSchemeMember_Request) (*memberpb.DeleteSchemeMember_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	result, err := h.DB.Conn.ExecContext(ctx, `
    DELETE FROM "scheme_member"
    USING "project_member", "project"
    WHERE "scheme_member"."project_member_id" = "project_member"."id"
      AND "project_member"."project_id" = "project"."id"
      AND "project"."owner_id" = $1
      AND "scheme_member"."scheme_id" = $2
      AND "scheme_member"."id" = $3
  `,
		in.GetOwnerId(),
		in.GetSchemeId(),
		in.GetSchemeMemberId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToDelete)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgMemberNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &memberpb.DeleteSchemeMember_Response{}, nil
}

// MembersWithoutScheme is ...
func (h *Handler) MembersWithoutScheme(ctx context.Context, in *memberpb.MembersWithoutScheme_Request) (*memberpb.MembersWithoutScheme_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &memberpb.MembersWithoutScheme_Response{}

	// Total count for pagination
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM
      "project_member"
      INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
      INNER JOIN "scheme" ON "project"."id" = "scheme"."project_id"
      INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
    WHERE
      "project_member"."id" NOT IN (
        SELECT "project_member_id"
        FROM "scheme_member"
        WHERE "scheme_id" = $2
      )
      AND "project"."owner_id" = $1
      AND "scheme"."id" = $2
      AND LOWER ( "user"."alias" ) LIKE LOWER ( '%' || $3 || '%' )
  `,
		in.GetOwnerId(),
		in.GetSchemeId(),
		in.GetAlias(),
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
      "project_member"."id",
      "user"."alias",
      "user"."email",
      "user"."name",
      "user"."surname",
      "project_member"."role",
      "project_member"."active",
      "project_member"."online"
    FROM
      "project_member"
      INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
      INNER JOIN "scheme" ON "project"."id" = "scheme"."project_id"
      INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
    WHERE
      "project_member"."id" NOT IN (
        SELECT "project_member_id"
        FROM "scheme_member"
        WHERE "scheme_id" = $2
      )
      AND "project"."owner_id" = $1
      AND "scheme"."id" = $2
      AND LOWER ( "user"."alias" ) LIKE LOWER ( '%' || $3 || '%' )
  `, sqlFooter)
	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery,
		in.GetOwnerId(),
		in.GetSchemeId(),
		in.GetAlias(),
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		member := &memberpb.MembersWithoutScheme_Member{}
		err = rows.Scan(
			&member.MemberId,
			&member.UserAlias,
			&member.Email,
			&member.UserName,
			&member.UserSurname,
			&member.Role,
			&member.Active,
			&member.Online,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}
		response.Members = append(response.Members, member)
	}

	return response, nil
}
