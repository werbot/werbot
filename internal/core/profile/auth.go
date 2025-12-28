package profile

import (
	"context"
	"fmt"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/core/notification"
	notificationenum "github.com/werbot/werbot/internal/core/notification/proto/enum"
	notificationmessage "github.com/werbot/werbot/internal/core/notification/proto/message"
	profilemessage "github.com/werbot/werbot/internal/core/profile/proto/message"
	"github.com/werbot/werbot/internal/core/token"
	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/utils/protoutils"
)

// SignIn function is used to authenticate the user by validating their credentials
// against the credentials stored in the database.
// It takes context and a SignIn_Request object as input and returns a Profile_Response object and an error response.
func (h *Handler) SignIn(ctx context.Context, in *profilemessage.SignIn_Request) (*profilemessage.Profile_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &profilemessage.Profile_Response{}
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
func (h *Handler) ResetPassword(ctx context.Context, in *profilemessage.ResetPassword_Request) (*profilemessage.ResetPassword_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	switch in.GetRequest().(type) {
	case *profilemessage.ResetPassword_Request_Email: // Step 1.1: Sending an email with a verification link
		// Get profile_id by email
		var profileID string
		err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT "id"
      FROM "profile"
      WHERE "email" = $1 AND "active" = true
    `, in.GetEmail()).Scan(&profileID)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		// Get or create token via token package
		tokenHandler := token.Handler{DB: h.DB, Worker: h.Worker}
		tokenID, isNew, err := tokenHandler.GetOrCreateProfileToken(ctx, profileID, tokenenum.Action_reset, func(ctx context.Context, profileID string) (string, error) {
			tokenResp, err := tokenHandler.AddTokenProfileReset(ctx, &tokenmessage.AddTokenProfileReset_Request{
				ProfileId: profileID,
			})
			if err != nil {
				return "", err
			}
			return tokenResp.GetToken(), nil
		})
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		// send email with token link
		notification := notification.Handler{DB: h.DB, Worker: h.Worker}
		if _, err := notification.SendMail(ctx, &notificationmessage.SendMail_Request{
			Email:    in.GetEmail(),
			Subject:  "reset password confirmation",
			Template: notificationenum.MailTemplate_password_reset,
			Data: map[string]string{
				"Link":      fmt.Sprintf("%s/auth/password_reset/%s", internal.GetString("APP_DSN", "http://localhost:5173"), tokenID),
				"FirstSend": strconv.FormatBool(isNew),
			},
		}); err != nil {
			return nil, trace.Error(err, log, nil)
		}

		return &profilemessage.ResetPassword_Response{
			ProfileId: profileID,
		}, nil

	case *profilemessage.ResetPassword_Request_Token: // Step 1.2: Verify token
		// Check token via token package
		tokenHandler := token.Handler{DB: h.DB, Worker: h.Worker}
		tokenData, err := tokenHandler.Token(ctx, &tokenmessage.Token_Request{
			IsAdmin: false,
			Token:   in.GetToken(),
		})
		if err != nil {
			return nil, trace.Error(status.Error(codes.InvalidArgument, trace.MsgTokenIsInvalid), log, nil)
		}

		// Verify token is for reset password and has correct status
		if tokenData.GetAction() != tokenenum.Action_reset || tokenData.GetStatus() != tokenenum.Status_sent {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgTokenIsInvalid)
			return nil, trace.Error(errGRPC, log, nil)
		}

		// Check token creation time (24 hours)
		if tokenData.GetCreatedAt() == nil {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgTokenIsInvalid)
			return nil, trace.Error(errGRPC, log, nil)
		}

	case *profilemessage.ResetPassword_Request_Password: // Step 2: Saving a new password
		// Check token via token package
		tokenHandler := token.Handler{DB: h.DB, Worker: h.Worker}
		tokenData, err := tokenHandler.Token(ctx, &tokenmessage.Token_Request{
			IsAdmin: false,
			Token:   in.GetPassword().GetToken(),
		})
		if err != nil {
			return nil, trace.Error(status.Error(codes.InvalidArgument, trace.MsgTokenIsInvalid), log, nil)
		}

		// Verify token is for reset password and has correct status
		if tokenData.GetAction() != tokenenum.Action_reset || tokenData.GetStatus() != tokenenum.Status_sent {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgTokenIsInvalid)
			return nil, trace.Error(errGRPC, log, nil)
		}

		profileID := tokenData.GetProfileId()
		if profileID == "" {
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

		// Update token status to used via token package
		// Use direct SQL within transaction, as UpdateProfileToken uses its own transaction
		_, err = tx.ExecContext(ctx, `
      UPDATE "token"
      SET "status" = $1
      WHERE "id" = $2
    `, tokenenum.Status_used, in.GetPassword().GetToken())
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if err := tx.Commit(); err != nil {
			return nil, trace.Error(err, log, trace.MsgTransactionCommitError)
		}
	}

	return &profilemessage.ResetPassword_Response{}, nil
}
