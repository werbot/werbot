package account

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal"
	accountpb "github.com/werbot/werbot/internal/grpc/account/proto/account"
	invitepb "github.com/werbot/werbot/internal/grpc/invite/proto/invite"
	"github.com/werbot/werbot/internal/grpc/notification"
	notificationpb "github.com/werbot/werbot/internal/grpc/notification/proto/notification"
	userpb "github.com/werbot/werbot/internal/grpc/user/proto/user"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/uuid"
)

// SignIn function is used to authenticate the user by validating their credentials
// against the credentials stored in the database.
// It takes context and a SignIn_Request object as input and returns a User_Response object and an error response.
func (h *Handler) SignIn(ctx context.Context, in *accountpb.SignIn_Request) (*userpb.User_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &userpb.User_Response{}
	response.Email = in.GetEmail()

	stmt, err := h.DB.Conn.PrepareContext(ctx, `
    SELECT
      "id",
      "alias",
      "name",
      "surname",
      "password",
      "active",
      "confirmed",
      "role"
    FROM "user"
    WHERE
      email = $1
      AND "active" = TRUE
      AND "confirmed" = TRUE
  `)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer stmt.Close()

	var password string
	err = stmt.QueryRowContext(ctx, in.GetEmail()).Scan(
		&response.UserId,
		&response.Alias,
		&response.Name,
		&response.Surname,
		&password,
		&response.Active,
		&response.Confirmed,
		&response.Role,
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	// Compare the hashed password retrieved from the database against the hashed password supplied in the request.
	if !crypto.CheckPasswordHash(in.GetPassword(), password) {
		errGRPC := status.Error(codes.NotFound, trace.MsgUserNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return response, nil
}

// ResetPassword is ...
func (h *Handler) ResetPassword(ctx context.Context, in *accountpb.ResetPassword_Request) (*accountpb.ResetPassword_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	switch in.GetRequest().(type) {
	case *accountpb.ResetPassword_Request_Email: // Step 1.1: Sending an email with a verification link
		var first bool
		var userID, token sql.NullString
		err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "user"."id",
      "user_token"."token"
    FROM "user"
      LEFT JOIN "user_token" ON "user"."id" = "user_token"."user_id"
        AND "user_token"."active" = true
        AND "user_token"."action" = 4
        AND "user_token"."created_at" > CURRENT_TIMESTAMP - INTERVAL '24 hour'
    WHERE
      "user"."email" = $1
      AND "user"."active" = true
  `, in.GetEmail()).Scan(
			&userID,
			&token)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if !token.Valid {
			first = true
			token.String = uuid.New()
			_, err = h.DB.Conn.ExecContext(ctx, `
        INSERT INTO "user_token" ("token", "user_id", "action")
        VALUES ($1, $2, $3)
      `,
				token.String,
				userID.String,
				invitepb.Action_reset.Enum(),
			)
			if err != nil {
				return nil, trace.Error(err, log, trace.MsgFailedToAdd)
			}
		}

		// send email with token link
		notification := notification.Handler{DB: h.DB, Worker: h.Worker}
		notification.SendMail(ctx, &notificationpb.SendMail_Request{
			Email:    in.GetEmail(),
			Subject:  "reset password confirmation",
			Template: notificationpb.MailTemplate_password_reset,
			Data: map[string]string{
				"Link":      fmt.Sprintf("%s/auth/password_reset/%s", internal.GetString("APP_DSN", "http://localhost:5173"), token.String),
				"FirstSend": strconv.FormatBool(first),
			},
		})

		return &accountpb.ResetPassword_Response{
			UserId: userID.String,
		}, nil

	case *accountpb.ResetPassword_Request_Token: // Step 1.2: Verify token
		var userID sql.NullString
		err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT "user_id"
      FROM "user_token"
      WHERE
        "token" = $1
        AND "active" = true
        AND "created_at" > CURRENT_TIMESTAMP - INTERVAL '24 hour'
    `, in.GetToken()).Scan(&userID)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if userID.Valid {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgInviteIsInvalid)
			return nil, trace.Error(errGRPC, log, nil)
		}

	case *accountpb.ResetPassword_Request_Password: // Step 2: Saving a new password
		var userID sql.NullString
		err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT "user_id"
      FROM "user_token"
      WHERE
        "token" = $1
        AND "active" = true
        AND "created_at" > CURRENT_TIMESTAMP - INTERVAL '24 hour'
    `, in.GetToken()).Scan(&userID)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if userID.Valid {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgInviteIsInvalid)
			return nil, trace.Error(errGRPC, log, nil)
		}

		newPassword, err := crypto.HashPassword(in.GetPassword().GetPassword(), internal.GetInt("PASSWORD_HASH_COST", 13))
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		tx, err := h.DB.Conn.BeginTx(ctx, nil)
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgTransactionCreateError)
		}
		defer tx.Rollback()

		_, err = tx.ExecContext(ctx, `
      UPDATE "user"
      SET "password" = $1
      WHERE "id" = $2
    `, newPassword, userID)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		_, err = tx.ExecContext(ctx, `
      UPDATE "user_token"
      SET "active" = TRUE,
      WHERE "token" = $1
    `, in.GetPassword().GetToken())
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if err := tx.Commit(); err != nil {
			return nil, trace.Error(err, log, trace.MsgTransactionCommitError)
		}
	}

	return &accountpb.ResetPassword_Response{}, nil
}
