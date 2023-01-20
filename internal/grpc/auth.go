package grpc

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	authpb "github.com/werbot/werbot/api/proto/auth"
	userpb "github.com/werbot/werbot/api/proto/user"
	"github.com/werbot/werbot/internal/crypto"
)

type auth struct {
	authpb.UnimplementedAuthHandlersServer
}

// SignIn is ...
func (a *auth) SignIn(ctx context.Context, in *authpb.SignIn_Request) (*userpb.User_Response, error) {
	var password string
	response := new(userpb.User_Response)
	response.Email = in.GetEmail()

	err := service.db.Conn.QueryRow(`SELECT "id", "login", "name", "surname", "password", "enabled", "confirmed", "role"
		    FROM "user"
		    WHERE "email" = $1
		      AND "enabled" = 't'
		      AND "confirmed" = 't'`,
		in.GetEmail(),
	).Scan(&response.UserId,
		&response.Login,
		&response.Name,
		&response.Surname,
		&password,
		&response.Enabled,
		&response.Confirmed,
		&response.Role,
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errNotFound
	}

	if !crypto.CheckPasswordHash(in.GetPassword(), password) {
		return nil, errPasswordIsNotValid
	}

	return response, nil
}

// ResetPassword is ...
func (a *auth) ResetPassword(ctx context.Context, in *authpb.ResetPassword_Request) (*authpb.ResetPassword_Response, error) {
	var userID, resetToken string
	response := new(authpb.ResetPassword_Response)

	// Sending an email with a verification link
	if in.GetEmail() != "" {
		// Check if there is a user with the specified email
		err := service.db.Conn.QueryRow(`SELECT "id" FROM "user" WHERE "email" = $1 AND "enabled" = 't'`,
			in.GetEmail(),
		).Scan(&userID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			service.log.FromGRPC(err).Send()
			return nil, errServerError
		}

		if userID == "" {
			response.Message = "Verification email has been sent"
			return response, nil
		}

		// Checking if a verification token has been sent in the last 24 hours
		resetToken, _ = tokenByUserID(userID, "reset")
		if len(resetToken) > 0 {
			response.Message = "Resend only after 24 hours"
			return response, nil
		}

		resetToken = uuid.New().String()
		data, err := service.db.Conn.Exec(`INSERT INTO "user_token" ("token", "user_id", "date_create", "action") VALUES ($1, $2, NOW(), 'reset')`,
			resetToken,
			userID,
		)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToAdd
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}

		response.Message = "Verification email has been sent"
		response.Token = resetToken
		return response, nil
	}

	// Checking the validity of a link
	if in.GetToken() != "" && in.GetPassword() == "" {
		if _, err := userIDByToken(in.GetToken()); err != nil {
			service.log.FromGRPC(err).Send()
			return nil, err
		}

		response.Message = "Token is valid"
		return response, nil
	}

	// Saving a new password
	if in.GetToken() != "" && in.GetPassword() != "" {
		id, err := userIDByToken(in.GetToken())
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, err
		}

		newPassword, err := crypto.HashPassword(in.GetPassword())
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errHashIsNotValid // HashPassword failed
		}

		tx, err := service.db.Conn.Begin()
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errTransactionCreateError
		}

		tx.Exec(`UPDATE "user" SET "password" = $1 WHERE "id" = $2`, newPassword, id)
		tx.Exec(`UPDATE "user_token" SET "used" = 't', date_used = NOW() WHERE "token" = $1`,
			in.GetToken(),
		)

		if err = tx.Commit(); err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errTransactionCommitError
		}

		/*
		   if _, err = db.Conn.Exec(`UPDATE "user" SET "password" = $1 WHERE "id" = $2`, newPassword, id); err != nil {
		     service.log.FromGRPC(err).Send()
		     return nil, errors.New("There was a problem updating")
		   }

		   if _, err = db.Conn.Exec(`UPDATE "user_token" SET "used" = 't' WHERE "token" = $1`, in.GetToken()); err != nil {
		     service.log.FromGRPC(err).Send()
		     return nil, errors.New("There was a problem updating")
		   }
		*/

		response.Message = "New password saved"
		return response, nil
	}

	return response, nil
}
