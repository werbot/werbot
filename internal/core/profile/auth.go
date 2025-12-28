package profile

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/core/notification"
	notificationpb "github.com/werbot/werbot/internal/core/notification/proto/notification"
	profilepb "github.com/werbot/werbot/internal/core/profile/proto/profile"
	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/uuid"
)

// SignIn function is used to authenticate the user by validating their credentials
// against the credentials stored in the database.
// It takes context and a SignIn_Request object as input and returns a Profile_Response object and an error response.
func (h *Handler) SignIn(ctx context.Context, in *profilepb.SignIn_Request) (*profilepb.Profile_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &profilepb.Profile_Response{}
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
    FROM "profile"
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
		&response.ProfileId,
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
func (h *Handler) ResetPassword(ctx context.Context, in *profilepb.ResetPassword_Request) (*profilepb.ResetPassword_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	switch in.GetRequest().(type) {
	case *profilepb.ResetPassword_Request_Email: // Step 1.1: Sending an email with a verification link
		var first bool
		var profileID, token sql.NullString
		err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "profile"."id",
      "profile_token"."token"
    FROM "profile"
      LEFT JOIN "profile_token" ON "profile"."id" = "profile_token"."profile_id"
        AND "profile_token"."active" = true
        AND "profile_token"."action" = 4
        AND "profile_token"."created_at" > CURRENT_TIMESTAMP - INTERVAL '24 hour'
    WHERE
      "profile"."email" = $1
      AND "profile"."active" = true
  `, in.GetEmail()).Scan(
			&profileID,
			&token)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if !token.Valid {
			first = true
			token.String = uuid.New()
			_, err = h.DB.Conn.ExecContext(ctx, `
        INSERT INTO "profile_token" ("token", "profile_id", "action")
        VALUES ($1, $2, $3)
      `,
				token.String,
				profileID.String,
				tokenenum.Action_reset.Enum(),
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

		return &profilepb.ResetPassword_Response{
			ProfileId: profileID.String,
		}, nil

	case *profilepb.ResetPassword_Request_Token: // Step 1.2: Verify token
		var profileID sql.NullString
		err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT "profile_id"
      FROM "profile_token"
      WHERE
        "token" = $1
        AND "active" = true
        AND "created_at" > CURRENT_TIMESTAMP - INTERVAL '24 hour'
    `, in.GetToken()).Scan(&profileID)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if profileID.Valid {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgTokenIsInvalid)
			return nil, trace.Error(errGRPC, log, nil)
		}

	case *profilepb.ResetPassword_Request_Password: // Step 2: Saving a new password
		var profileID sql.NullString
		err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT "profile_id"
      FROM "profile_token"
      WHERE
        "token" = $1
        AND "active" = true
        AND "created_at" > CURRENT_TIMESTAMP - INTERVAL '24 hour'
    `, in.GetToken()).Scan(&profileID)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if profileID.Valid {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgTokenIsInvalid)
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
      UPDATE "profile"
      SET "password" = $1
      WHERE "id" = $2
    `, newPassword, profileID)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		_, err = tx.ExecContext(ctx, `
      UPDATE "profile_token"
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

	return &profilepb.ResetPassword_Response{}, nil
}
