package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal"
	invitepb "github.com/werbot/werbot/internal/core/invite/proto/invite"
	"github.com/werbot/werbot/internal/core/notification"
	notificationpb "github.com/werbot/werbot/internal/core/notification/proto/notification"
	userpb "github.com/werbot/werbot/internal/core/user/proto/user"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
	"github.com/werbot/werbot/pkg/uuid"
)

// Users is lists all users on the system
func (h *Handler) Users(ctx context.Context, in *userpb.Users_Request) (*userpb.Users_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &userpb.Users_Response{}

	// Total count for pagination
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM "user"
  `).Scan(&response.Total)
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
      "user"."id",
      "user"."alias",
      "user"."name",
      "user"."surname",
      "user"."email",
      "user"."active",
      "user"."confirmed",
      "user"."role",
      "user"."locked_at",
      "user"."archived_at",
      "user"."updated_at",
      "user"."created_at",
      COUNT(DISTINCT "project"."id") AS "count_project",
      COUNT(DISTINCT "user_public_key"."id") AS "count_keys",
      COUNT(DISTINCT "scheme"."id") AS "count_schemes"
    FROM "user"
    LEFT JOIN "project" ON "project"."owner_id" = "user"."id"
    LEFT JOIN "user_public_key" ON "user_public_key"."user_id" = "user"."id"
    LEFT JOIN "scheme" ON "scheme"."project_id" = "project"."id"
    GROUP BY
      "user"."id",
      "user"."alias",
      "user"."name",
      "user"."surname",
      "user"."email",
      "user"."active",
      "user"."confirmed",
      "user"."role",
      "user"."locked_at",
      "user"."archived_at",
      "user"."updated_at",
      "user"."created_at"
  `, sqlFooter)
	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		var lockedAt, archivedAt, updatedAt, createdAt pgtype.Timestamp
		user := &userpb.User_Response{}
		err = rows.Scan(
			&user.UserId,
			&user.Alias,
			&user.Name,
			&user.Surname,
			&user.Email,
			&user.Active,
			&user.Confirmed,
			&user.Role,
			&lockedAt,
			&archivedAt,
			&updatedAt,
			&createdAt,
			&user.ProjectsCount,
			&user.KeysCount,
			&user.SchemesCount,
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
			ghoster.Secrets(user, true)
		}

		response.Users = append(response.Users, user)
	}

	return response, nil
}

// User is displays user information
func (h *Handler) User(ctx context.Context, in *userpb.User_Request) (*userpb.User_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	var lockedAt, archivedAt, updatedAt, createdAt pgtype.Timestamp
	response := &userpb.User_Response{}
	response.UserId = in.GetUserId()

	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "user"."alias",
      "user"."name",
      "user"."surname",
      "user"."email",
      "user"."active",
      "user"."confirmed",
      "user"."role",
      "user"."locked_at",
      "user"."archived_at",
      "user"."updated_at",
      "user"."created_at",
      COUNT(DISTINCT "project"."id") AS "count_project",
      COUNT(DISTINCT "user_public_key"."id") AS "count_keys",
      COUNT(DISTINCT "scheme"."id") AS "count_schemes"
    FROM "user"
    LEFT JOIN "project" ON "project"."owner_id" = "user"."id"
    LEFT JOIN "user_public_key" ON "user_public_key"."user_id" = "user"."id"
    LEFT JOIN "scheme" ON "scheme"."project_id" = "project"."id"
    WHERE "user"."id" = $1
    GROUP BY
      "user"."alias",
      "user"."name",
      "user"."surname",
      "user"."email",
      "user"."active",
      "user"."confirmed",
      "user"."role",
      "user"."locked_at",
      "user"."archived_at",
      "user"."updated_at",
      "user"."created_at"
  `, in.GetUserId(),
	).Scan(
		&response.Alias,
		&response.Name,
		&response.Surname,
		&response.Email,
		&response.Active,
		&response.Confirmed,
		&response.Role,
		&lockedAt,
		&archivedAt,
		&updatedAt,
		&createdAt,
		&response.ProjectsCount,
		&response.KeysCount,
		&response.SchemesCount,
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

// AddUser is adds a new user
func (h *Handler) AddUser(ctx context.Context, in *userpb.AddUser_Request) (*userpb.AddUser_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &userpb.AddUser_Response{}
	password, _ := crypto.HashPassword(in.Password, internal.GetInt("PASSWORD_HASH_COST", 13))
	err := h.DB.Conn.QueryRowContext(ctx, `
    INSERT INTO "user" ("alias", "name", "surname", "email", "password", "active", "confirmed")
    SELECT $1, $2, $3, $4::varchar(64), $5, $6, $7
    WHERE NOT EXISTS (
      SELECT 1 FROM "user" WHERE "email" = $4
    )
    RETURNING "id"
  `,
		in.GetAlias(),
		in.GetName(),
		in.GetSurname(),
		in.GetEmail(),
		password,
		in.GetActive(),
		in.GetConfirmed(),
	).Scan(&response.UserId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errGRPC := status.Error(codes.Canceled, trace.MsgFailedToAdd)
			return nil, trace.Error(errGRPC, log, nil)
		}
		return nil, trace.Error(err, log, "")
	}

	return response, nil
}

// UpdateUser is updates user data
func (h *Handler) UpdateUser(ctx context.Context, in *userpb.UpdateUser_Request) (*userpb.UpdateUser_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	var column string
	var value any

	switch setting := in.GetSetting().(type) {
	case *userpb.UpdateUser_Request_Name:
		column = "name"
		value = in.GetName()
	case *userpb.UpdateUser_Request_Surname:
		column = "surname"
		value = in.GetSurname()

	case *userpb.UpdateUser_Request_Alias,
		*userpb.UpdateUser_Request_Email,
		*userpb.UpdateUser_Request_Confirmed,
		*userpb.UpdateUser_Request_Active,
		*userpb.UpdateUser_Request_Archived:
		if !in.GetIsAdmin() {
			errGRPC := status.Error(codes.InvalidArgument, "setting: exactly one field is required in oneof")
			return nil, trace.Error(errGRPC, log, nil)
		}

		switch setting.(type) {
		case *userpb.UpdateUser_Request_Alias:
			column = "alias"
			value = in.GetAlias()
		case *userpb.UpdateUser_Request_Email:
			column = "email"
			value = in.GetEmail()
		case *userpb.UpdateUser_Request_Confirmed:
			column = "confirmed"
			value = in.GetConfirmed()
		case *userpb.UpdateUser_Request_Active:
			column = "active"
			value = in.GetActive()
		case *userpb.UpdateUser_Request_Archived:
			column = "archived"
			value = in.GetArchived()
		}
	}

	query := fmt.Sprintf(`
    UPDATE "user"
    SET "%s" = $1
    WHERE "id" = $2
  `, column)

	result, err := h.DB.Conn.ExecContext(ctx, query, value, in.GetUserId())
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgUserNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &userpb.UpdateUser_Response{}, nil
}

// DeleteUser is ...
func (h *Handler) DeleteUser(ctx context.Context, in *userpb.DeleteUser_Request) (*userpb.DeleteUser_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	notification := notification.Handler{DB: h.DB, Worker: h.Worker}

	switch in.GetRequest().(type) {
	case *userpb.DeleteUser_Request_Password: // step 1
		var first bool
		var alias, passwordHash, email string
		var token sql.NullString
		err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT
        "user"."alias",
        "user"."password",
        "user"."email",
        "user_token"."token"
      FROM
        "user"
        LEFT JOIN "user_token" ON "user"."id" = "user_token"."user_id"
          AND "user_token"."active" = true
          AND "user_token"."action" = 5
          AND "user_token"."created_at" > CURRENT_TIMESTAMP - INTERVAL '24 hour'
      WHERE
        "user"."id" = $1
        AND "user"."role" = 1
    `, in.GetUserId()).Scan(
			&alias,
			&passwordHash,
			&email,
			&token,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if !crypto.CheckPasswordHash(in.GetPassword(), passwordHash) {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgPasswordIsNotValid)
			return nil, trace.Error(errGRPC, log, nil)
		}

		if !token.Valid {
			first = true
			token.String = uuid.New()
			_, err = h.DB.Conn.ExecContext(ctx, `
        INSERT INTO "user_token" ("token", "user_id", "action")
        VALUES ($1, $2, $3)
      `,
				token.String,
				in.GetUserId(),
				invitepb.Action_delete.Enum(),
			)
			if err != nil {
				return nil, trace.Error(err, log, trace.MsgFailedToAdd)
			}
		}

		// send email with token link
		_, err = notification.SendMail(ctx, &notificationpb.SendMail_Request{
			Email:    email,
			Subject:  "user deletion confirmation",
			Template: notificationpb.MailTemplate_account_deletion_confirmation,
			Data: map[string]string{
				"Login":     alias,
				"Link":      fmt.Sprintf("%s/profile/setting/destroy/%s", internal.GetString("APP_DSN", "http://localhost:5173"), token),
				"FirstSend": strconv.FormatBool(first),
			},
		})

	case *userpb.DeleteUser_Request_Token: // step 2
		var alias, email sql.NullString
		err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT "user"."alias", "user"."email"
      FROM "user"
      JOIN "user_token" ON "user"."id" = "user_token"."user_id"
      WHERE
        "user_token"."user_id" = $1::uuid
        AND "user_token"."token" = $2::uuid
        AND "user_token"."active" = true
        AND "user_token"."created_at" > CURRENT_TIMESTAMP - INTERVAL '24 hour'
        AND "user"."role" = 1
    `,
			in.GetUserId(),
			in.GetToken(),
		).Scan(
			&alias,
			&email,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if !alias.Valid || !email.Valid {
			errGRPC := status.Error(codes.InvalidArgument, "")
			return nil, trace.Error(errGRPC, log, nil)
		}

		tx, err := h.DB.Conn.BeginTx(ctx, nil)
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgTransactionCreateError)
		}
		defer tx.Rollback()

		// disable user
		archivedAt := time.Now().AddDate(0, 1, 0)

		_, err = tx.ExecContext(ctx, `
      UPDATE "user"
      SET
        "active" = false,
        "locked_at" = NOW(),
        "archived_at" = $2
      WHERE "id" = $1
    `,
			in.GetUserId(),
			archivedAt,
		)
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
		}

		// disable user user
		_, err = tx.ExecContext(ctx, `
      UPDATE "user_token"
      SET "active" = false
      WHERE "token" = $1
    `, in.GetToken())
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
		}

		if err := tx.Commit(); err != nil {
			return nil, trace.Error(err, log, trace.MsgTransactionCommitError)
		}

		// send delete token to email
		notification.SendMail(ctx, &notificationpb.SendMail_Request{
			Email:    email.String,
			Subject:  "user deleted",
			Template: notificationpb.MailTemplate_account_deletion_info,
			Data: map[string]string{
				"Login": alias.String,
			},
		})
	}

	return &userpb.DeleteUser_Response{}, nil
}

// UpdatePassword is ...
func (h *Handler) UpdatePassword(ctx context.Context, in *userpb.UpdatePassword_Request) (*userpb.UpdatePassword_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	// Check for a validity of the old password
	var currentPassword string
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT "password"
    FROM "user"
    WHERE "id" = $1
  `, in.GetUserId()).Scan(&currentPassword)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	if !crypto.CheckPasswordHash(in.GetOldPassword(), currentPassword) {
		errGRPC := status.Error(codes.InvalidArgument, trace.MsgPasswordIsNotValid)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// We change the old password for a new
	newPassword, err := crypto.HashPassword(in.GetNewPassword(), internal.GetInt("PASSWORD_HASH_COST", 13))
	if err != nil {
		errGRPC := status.Error(codes.InvalidArgument, trace.MsgPasswordIsNotValid)
		return nil, trace.Error(errGRPC, log, nil)
	}

	result, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "user"
    SET "password" = $1
    WHERE "id" = $2
  `,
		newPassword,
		in.GetUserId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgUserNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &userpb.UpdatePassword_Response{}, nil
}
