package user

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/werbot/werbot/internal/crypto"
	userpb "github.com/werbot/werbot/internal/grpc/user/proto"
	"github.com/werbot/werbot/internal/trace"
)

// ListUsers is lists all users on the system
func (h *Handler) ListUsers(ctx context.Context, in *userpb.ListUsers_Request) (*userpb.ListUsers_Response, error) {
	response := &userpb.ListUsers_Response{}

	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := h.DB.Conn.QueryContext(ctx, `
    SELECT
      "id",
      "login",
      "name",
      "surname",
      "email",
      "enabled",
      "confirmed",
      "updated_at",
      "created_at",
      "role",
      (
        SELECT COUNT(*)
        FROM "project"
        WHERE "owner_id" = "user"."id"
      ) AS "count_project",
      (
        SELECT COUNT(*)
        FROM "user_public_key"
        WHERE "user_id" = "user"."id"
      ) AS "count_keys",
      (
        SELECT COUNT(*)
        FROM
          "project"
          JOIN "server" ON "project"."id" = "server"."project_id"
        WHERE "project"."owner_id" = "user"."id"
      ) AS "count_servers"
    FROM "user"
  `+sqlFooter)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	for rows.Next() {
		var countServers, countProjects, countKeys int32
		var updateAt, createdAt pgtype.Timestamp
		user := &userpb.User_Response{}
		userDetail := &userpb.ListUsers_Response_UserInfo{}
		err = rows.Scan(&user.UserId,
			&user.Login,
			&user.Name,
			&user.Surname,
			&user.Email,
			&user.Enabled,
			&user.Confirmed,
			&updateAt,
			&createdAt,
			&user.Role,
			&countProjects,
			&countKeys,
			&countServers,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		user.UpdatedAt = timestamppb.New(updateAt.Time)
		user.CreatedAt = timestamppb.New(createdAt.Time)

		userDetail.ServersCount = countServers
		userDetail.ProjectsCount = countProjects
		userDetail.KeysCount = countKeys
		userDetail.User = user

		response.Users = append(response.Users, userDetail)
	}
	defer rows.Close()

	// Total count for pagination
	err = h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM "user"
  `).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		return nil, trace.Error(err, log, nil)
	}

	return response, nil
}

// User is displays user information
func (h *Handler) User(ctx context.Context, in *userpb.User_Request) (*userpb.User_Response, error) {
	var updateAt, createdAt pgtype.Timestamp
	response := &userpb.User_Response{}
	response.UserId = in.GetUserId()

	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "login",
      "name",
      "surname",
      "email",
      "enabled",
      "confirmed",
      "role",
      "updated_at",
      "created_at"
    FROM "user"
    WHERE "id" = $1
  `, in.GetUserId(),
	).Scan(&response.Login,
		&response.Name,
		&response.Surname,
		&response.Email,
		&response.Enabled,
		&response.Confirmed,
		&response.Role,
		&updateAt,
		&createdAt,
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	response.UpdatedAt = timestamppb.New(updateAt.Time)
	response.CreatedAt = timestamppb.New(createdAt.Time)

	return response, nil
}

// AddUser is adds a new user
func (h *Handler) AddUser(ctx context.Context, in *userpb.AddUser_Request) (*userpb.AddUser_Response, error) {
	response := &userpb.AddUser_Response{}

	tx, err := h.DB.Conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCreateError)
	}
	defer tx.Rollback()

	// Checking if the email address already exists
	err = tx.QueryRowContext(ctx, `
    SELECT "id"
    FROM "user"
    WHERE "email" = $1
  `, in.GetEmail(),
	).Scan(&response.UserId)
	if err != nil && err != sql.ErrNoRows {
		return nil, trace.Error(err, log, nil)
	}

	if response.UserId != "" {
		errGRPC := status.Error(codes.AlreadyExists, "")
		return nil, trace.Error(errGRPC, log, nil)
	}

	// Adds a new entry to the database
	password, _ := crypto.HashPassword(in.Password)
	err = tx.QueryRowContext(ctx, `
    INSERT INTO
      "user" (
        "login",
        "name",
        "surname",
        "email",
        "password",
        "enabled",
        "confirmed"
      )
    VALUES ($1, $2, $3, $4, $5, $6)
    RETURNING "id"
  `,
		in.GetLogin(),
		in.GetName(),
		in.GetSurname(),
		in.GetEmail(),
		password,
		in.GetEnabled(),
		in.GetConfirmed(),
	).Scan(&response.UserId)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
	}

	if err := tx.Commit(); err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCommitError)
	}

	return response, nil
}

// UpdateUser is updates user data
func (h *Handler) UpdateUser(ctx context.Context, in *userpb.UpdateUser_Request) (*userpb.UpdateUser_Response, error) {
	var err error
	response := &userpb.UpdateUser_Response{}

	switch in.GetRequest().(type) {
	case *userpb.UpdateUser_Request_Info:
		_, err = h.DB.Conn.ExecContext(ctx, `
      UPDATE "user"
      SET
        "login" = $1,
        "email" = $2,
        "name" = $3,
        "surname" = $4
      WHERE "id" = $5
    `,
			in.GetInfo().GetLogin(),
			in.GetInfo().GetEmail(),
			in.GetInfo().GetName(),
			in.GetInfo().GetSurname(),
			in.GetUserId(),
		)
	case *userpb.UpdateUser_Request_Confirmed:
		_, err = h.DB.Conn.ExecContext(ctx, `
      UPDATE "user"
      SET "confirmed" = $1
      WHERE "id" = $2
    `, in.GetConfirmed(), in.GetUserId())
	case *userpb.UpdateUser_Request_Enabled:
		_, err = h.DB.Conn.ExecContext(ctx, `
      UPDATE "user"
      SET "enabled" = $1
      WHERE "id" = $2
    `, in.GetEnabled(), in.GetUserId())
	default:
		errGRPC := status.Error(codes.InvalidArgument, "")
		return nil, trace.Error(errGRPC, log, nil)
	}

	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	return response, nil
}

// DeleteUser is ...
func (h *Handler) DeleteUser(ctx context.Context, in *userpb.DeleteUser_Request) (*userpb.DeleteUser_Response, error) {
	var login, passwordHash, email string
	response := &userpb.DeleteUser_Response{}

	switch in.GetRequest().(type) {
	case *userpb.DeleteUser_Request_Password:
		tx, err := h.DB.Conn.BeginTx(ctx, nil)
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgTransactionCreateError)
		}
		defer tx.Rollback()

		err = tx.QueryRowContext(ctx, `
      SELECT
        "login",
        "password",
        "email"
      FROM "user"
      WHERE "id" = $1
    `, in.GetUserId(),
		).Scan(&login,
			&passwordHash,
			&email,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if !crypto.CheckPasswordHash(in.GetPassword(), passwordHash) {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgPasswordIsNotValid)
			return nil, trace.Error(errGRPC, log, nil)
		}

		// Checking if a verification token has been sent in the last 24 hours
		deleteToken, _ := TokenByUserID(ctx, h, in.GetUserId(), "delete")
		if len(deleteToken) > 0 {
			response.Login = login
			response.Email = email
			response.Token = deleteToken
			return response, nil
		}

		deleteToken = uuid.New().String()
		_, err = tx.ExecContext(ctx, `
      INSERT INTO "user_token" ("token", "user_id", "action")
      VALUES ($1, $2, 'delete')
    `, deleteToken, in.GetUserId())
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToAdd)
		}

		if err := tx.Commit(); err != nil {
			return nil, trace.Error(err, log, trace.MsgTransactionCommitError)
		}

		response.Email = email
		response.Token = deleteToken
		return response, nil

	case *userpb.DeleteUser_Request_Token:
		userID, _ := UserIDByToken(ctx, h, in.GetToken())
		if userID != in.GetUserId() {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgInviteIsInvalid)
			return nil, trace.Error(errGRPC, log, nil)
		}

		tx, err := h.DB.Conn.BeginTx(ctx, nil)
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgTransactionCreateError)
		}
		defer tx.Rollback()

		_, err = tx.ExecContext(ctx, `
      UPDATE "user"
      SET "enabled" = 'f'
      WHERE "id" = $1
    `, in.GetUserId())
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
		}

		_, err = tx.ExecContext(ctx, `
      UPDATE "user_token"
      SET
        "used" = 't',
        "date_used" = NOW()
      WHERE "token" = $1
    `, in.GetToken())
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
		}

		err = tx.QueryRowContext(ctx, `
      SELECT
        "login",
        "email"
      FROM "user"
      WHERE "id" = $1
    `, in.GetUserId()).Scan(&login,
			&email,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if err := tx.Commit(); err != nil {
			return nil, trace.Error(err, log, trace.MsgTransactionCommitError)
		}

		response.Login = login
		response.Email = email
		return response, nil
	}

	errGRPC := status.Error(codes.InvalidArgument, "")
	return nil, trace.Error(errGRPC, log, nil)
}

// UpdatePassword is ...
func (h *Handler) UpdatePassword(ctx context.Context, in *userpb.UpdatePassword_Request) (*userpb.UpdatePassword_Response, error) {
	response := &userpb.UpdatePassword_Response{}

	if len(in.GetOldPassword()) > 0 {
		// Check for a validity of the old password
		var currentPassword string
		err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT "password"
      FROM "user"
      WHERE "id" = $1
    `, in.GetUserId(),
		).Scan(&currentPassword)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if !crypto.CheckPasswordHash(in.GetOldPassword(), currentPassword) {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgPasswordIsNotValid)
			return nil, trace.Error(errGRPC, log, nil)
		}
	}

	// We change the old password for a new
	newPassword, err := crypto.HashPassword(in.GetNewPassword())
	if err != nil {
		errGRPC := status.Error(codes.InvalidArgument, trace.MsgPasswordIsNotValid)
		return nil, trace.Error(errGRPC, log, nil)
	}

	_, err = h.DB.Conn.ExecContext(ctx, `
    UPDATE "user"
    SET "password" = $1
    WHERE "id" = $2
  `, newPassword, in.GetUserId())
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	return response, nil
}

// TokenByUserID is ...
func TokenByUserID(ctx context.Context, h *Handler, userID, action string) (string, error) {
	var token string
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT "token"
    FROM "user_token"
    WHERE
      "user_id" = $1
      AND "used" = 'f'
      AND "action" = $2
      AND "created_at" > NOW() - INTERVAL '24 hour'
  `, userID, action,
	).Scan(&token)
	if err != nil {
		return "", trace.Error(err, log, nil)
	}

	if token == "" {
		errGRPC := status.Error(codes.InvalidArgument, trace.MsgInviteIsInvalid)
		return token, trace.Error(errGRPC, log, nil)
	}

	return token, nil
}

// UserIDByToken is ...
func UserIDByToken(ctx context.Context, h *Handler, token string) (string, error) {
	var id string
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT "user_id" AS "id"
    FROM "user_token"
    WHERE
      "token" = $1
      AND "used" = 'f'
      AND "created_at" > NOW() - INTERVAL '24 hour'
  `, token,
	).Scan(&id)
	if err != nil {
		return "", trace.Error(err, log, nil)
	}

	if id == "" {
		errGRPC := status.Error(codes.InvalidArgument, trace.MsgInviteIsInvalid)
		return id, trace.Error(errGRPC, log, nil)
	}

	return id, nil
}
