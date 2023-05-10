package account

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"

	"github.com/werbot/werbot/internal/crypto"
	accountpb "github.com/werbot/werbot/internal/grpc/account/proto"
	"github.com/werbot/werbot/internal/grpc/user"
	userpb "github.com/werbot/werbot/internal/grpc/user/proto"
	"github.com/werbot/werbot/internal/trace"
)

// SignIn function is used to authenticate the user by validating their credentials
// against the credentials stored in the database.
// It takes context and a SignIn_Request object as input and returns a User_Response object and an error response.
func (h *Handler) SignIn(ctx context.Context, in *accountpb.SignIn_Request) (*userpb.User_Response, error) {
	response := new(userpb.User_Response)
	response.Email = in.GetEmail()

	stmt, err := h.DB.Conn.PrepareContext(ctx, `SELECT "id", "login", "name", "surname", "password", "enabled", "confirmed", "role"
    FROM "user"
    WHERE "email" = $1
      AND "enabled" = 't'
      AND "confirmed" = 't'`)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log)
	}
	defer stmt.Close()

	var password string
	err = stmt.QueryRowContext(ctx, in.GetEmail()).Scan(&response.UserId,
		&response.Login,
		&response.Name,
		&response.Surname,
		&password,
		&response.Enabled,
		&response.Confirmed,
		&response.Role,
	)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	// Compare the hashed password retrieved from the database against the hashed password supplied in the request.
	if !crypto.CheckPasswordHash(in.GetPassword(), password) {
		return nil, trace.Error(codes.InvalidArgument, trace.MsgPasswordIsNotValid)
	}

	return response, nil
}

// ResetPassword is ...
func (h *Handler) ResetPassword(ctx context.Context, in *accountpb.ResetPassword_Request) (*accountpb.ResetPassword_Response, error) {
	response := new(accountpb.ResetPassword_Response)

	switch {
	// Sending an email with a verification link
	case in.GetEmail() != "":
		var userID, resetToken sql.NullString
		err := h.DB.Conn.QueryRowContext(ctx, `SELECT "id" FROM "user" WHERE "email" = $1 AND "enabled" = 't'`,
			in.GetEmail(),
		).Scan(&userID)
		if err != nil {
			return nil, trace.ErrorAborted(err, h.Log)
		}

		//if userID.Valid {
		//	response.Message = "Verification email has been sent"
		//	return response, nil
		//}

		// Checking if a verification token has been sent in the last 24 hours
		resetToken.String, _ = user.TokenByUserID(ctx, &user.Handler{
			DB: h.DB,
		}, userID.String, "reset")
		if len(resetToken.String) > 0 {
			response.Message = "Resend only after 24 hours"
			return response, nil
		}

		resetToken.String = uuid.New().String()
		_, err = h.DB.Conn.ExecContext(ctx, `INSERT INTO "user_token" ("token", "user_id", "action") VALUES ($1, $2, 'reset')`,
			resetToken.String,
			userID.String,
		)
		if err != nil {
			return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToAdd)
		}

		response.Message = "Verification email has been sent"
		response.Token = resetToken.String
		return response, nil

	// Checking the validity of a link
	case in.GetToken() != "" && in.GetPassword() == "":
		_, err := user.UserIDByToken(ctx, &user.Handler{DB: h.DB}, in.GetToken())
		if err != nil {
			errorInfo := trace.ParseError(err)
			if errorInfo.Code == codes.NotFound {
				return nil, trace.Error(codes.NotFound, errorInfo.Message)
			}
			return nil, trace.ErrorAborted(err, h.Log)
		}

		response.Message = "Token is valid"
		return response, nil

	// Saving a new password
	case in.GetToken() != "" && in.GetPassword() != "":
		id, err := user.UserIDByToken(ctx, &user.Handler{DB: h.DB}, in.GetToken())
		if err != nil {
			errorInfo := trace.ParseError(err)
			if errorInfo.Code == codes.NotFound {
				return nil, trace.Error(codes.NotFound, errorInfo.Message)
			}
			return nil, trace.ErrorAborted(err, h.Log)
		}

		newPassword, err := crypto.HashPassword(in.GetPassword())
		if err != nil {
			return nil, trace.ErrorAborted(err, h.Log)
		}

		tx, err := h.DB.Conn.BeginTx(ctx, nil)
		if err != nil {
			return nil, trace.ErrorAborted(err, h.Log, trace.MsgTransactionCreateError)
		}
		defer tx.Rollback()

		_, err = tx.ExecContext(ctx, `UPDATE "user" SET "password" = $1, "last_update" = NOW() WHERE "id" = $2`, newPassword, id)
		if err != nil {
			return nil, trace.ErrorAborted(err, h.Log)
		}

		_, err = tx.ExecContext(ctx, `UPDATE "user_token" SET "used" = 't', date_used = NOW() WHERE "token" = $1`, in.GetToken())
		if err != nil {
			return nil, trace.ErrorAborted(err, h.Log)
		}

		if err := tx.Commit(); err != nil {
			return nil, trace.ErrorAborted(err, h.Log, trace.MsgTransactionCommitError)
		}

		response.Message = "New password saved"
		return response, nil
	}

	return response, nil
}
