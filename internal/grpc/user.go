package grpc

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/werbot/werbot/internal/crypto"

	pb_user "github.com/werbot/werbot/api/proto/user"
)

type user struct {
	pb_user.UnimplementedUserHandlersServer
}

// ListUsers is lists all users on the system
func (u *user) ListUsers(ctx context.Context, in *pb_user.ListUsers_Request) (*pb_user.ListUsers_Response, error) {
	sqlSearch := service.db.SQLAddWhere(in.GetQuery())
	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := service.db.Conn.Query(`SELECT
      "id",
      "fio",
      "name",
      "email",
      "enabled",
      "confirmed",
      "last_active",
      "register_date",
      "role",
      (SELECT COUNT(*) FROM "project" WHERE "owner_id" = "user"."id") AS "count_project",
      (SELECT COUNT(*) FROM "user_public_key" WHERE "user_id" = "user"."id") AS "count_keys",
      (SELECT COUNT(*) FROM "project" JOIN "server" ON "project"."id"="server"."project_id" WHERE "project"."owner_id"="user"."id") AS "count_servers"
    FROM
      "user"` + sqlSearch + sqlFooter)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToSelect
	}

	users := []*pb_user.ListUsers_Response_UserInfo{}
	for rows.Next() {
		var countServers, countProjects, countKeys int32
		var lastActive, registerDate pgtype.Timestamp
		user := new(pb_user.User_Response)
		err = rows.Scan(
			&user.UserId,
			&user.Fio,
			&user.Name,
			&user.Email,
			&user.Enabled,
			&user.Confirmed,
			&lastActive,
			&registerDate,
			&user.Role,
			&countProjects,
			&countKeys,
			&countServers,
		)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToScan
		}
		user.LastActive = timestamppb.New(lastActive.Time)
		user.RegisterDate = timestamppb.New(registerDate.Time)
		users = append(users, &pb_user.ListUsers_Response_UserInfo{
			ServersCount:  countServers,
			ProjectsCount: countProjects,
			KeysCount:     countKeys,
			User:          user,
		})
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	err = service.db.Conn.QueryRow(`SELECT COUNT (*) FROM "user"` + sqlSearch).
		Scan(&total)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToScan
	}

	return &pb_user.ListUsers_Response{
		Total: total,
		Users: users,
	}, nil
}

// User is displays user information
func (u *user) User(ctx context.Context, in *pb_user.User_Request) (*pb_user.User_Response, error) {
	user := new(pb_user.User_Response)
	user.UserId = in.GetUserId()

	err := service.db.Conn.QueryRow(`SELECT
      "fio",
      "name",
      "email",
      "enabled",
      "confirmed",
      "role"
    FROM
      "user"
    WHERE
      "id" = $1`,
		in.GetUserId(),
	).Scan(
		&user.Fio,
		&user.Name,
		&user.Email,
		&user.Enabled,
		&user.Confirmed,
		&user.Role,
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToScan
	}

	return user, nil
}

// AddUser is adds a new user
func (u *user) AddUser(ctx context.Context, in *pb_user.AddUser_Request) (*pb_user.AddUser_Response, error) {
	tx, err := service.db.Conn.Beginx()
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errTransactionCreateError
	}

	// Checking if the email address already exists
	var id string
	err = tx.QueryRow(`SELECT
			"id"
		FROM
			"user"
		WHERE
			"email" = $1`,
		in.GetEmail(),
	).Scan(&id)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToScan
	}
	if id != "" {
		return nil, errObjectAlreadyExists
	}

	// Adds a new entry to the database
	password, _ := crypto.HashPassword(in.Password)
	err = tx.QueryRow(`INSERT
	INTO "user" (
		"fio",
		"name",
		"email",
		"password",
		"enabled",
		"confirmed",
		"register_date"
	)
	VALUES
		($1, $2, $3, $4, $5, $6, NOW( ))
	RETURNING "id"`,
		in.GetFio(),
		in.GetName(),
		in.GetEmail(),
		password,
		in.GetEnabled(),
		in.GetConfirmed(),
	).Scan(&id)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	if err = tx.Commit(); err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errTransactionCommitError
	}

	return &pb_user.AddUser_Response{
		UserId: id,
	}, nil
}

// UpdateUser is updates user data
func (u *user) UpdateUser(ctx context.Context, in *pb_user.UpdateUser_Request) (*pb_user.UpdateUser_Response, error) {
	cnt := 0
	qParts := make([]string, 0, 2)
	args := make([]any, 0, 2)

	md := in.ProtoReflect()
	md.Range(func(fd protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		name := proto.GetExtension(fd.Options().(*descriptorpb.FieldOptions), pb_user.E_SqlName).(string)
		if name != "id" {
			cnt++
			qParts = append(qParts, fmt.Sprintf(`"%s" = $%v`, name, cnt))
			args = append(args, value)
		}
		return true
	})

	cnt++
	args = append(args, in.GetUserId())
	query := fmt.Sprintf(`UPDATE "user" SET %s WHERE "id" = $%v`,
		strings.Join(qParts, ", "),
		cnt,
	)
	data, err := service.db.Conn.Exec(query, args...)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_user.UpdateUser_Response{}, nil
}

// DeleteUser is ...
func (u *user) DeleteUser(ctx context.Context, in *pb_user.DeleteUser_Request) (*pb_user.DeleteUser_Response, error) {
	var name, passwordHash, email, deleteToken string
	deleteUser := new(pb_user.DeleteUser_Response)
	if in.GetPassword() != "" && in.GetUserId() != "" {
		err := service.db.Conn.QueryRow(`SELECT
				"name",
				"password",
				"email"
			FROM
				"user"
			WHERE
				"id" = $1`,
			in.GetUserId(),
		).Scan(&name, &passwordHash, &email)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToDelete
		}
		if !crypto.CheckPasswordHash(in.GetPassword(), passwordHash) {
			return nil, errPasswordIsNotValid
		}

		// Checking if a verification token has been sent in the last 24 hours
		deleteToken, _ = u.getTokenByUserID(in.GetUserId(), "delete")
		if len(deleteToken) > 0 {
			deleteUser.Name = name
			deleteUser.Email = email
			deleteUser.Token = deleteToken
			return deleteUser, nil
		}

		deleteToken = uuid.New().String()
		data, err := service.db.Conn.Exec(`INSERT
			INTO "user_token" (
				"token",
				"user_id",
				"date_create",
				"action"
			)
			VALUES
				($1, $2, NOW(), 'delete')`,
			deleteToken,
			in.GetUserId(),
		)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, err // Create delete token failed
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}

		deleteUser.Email = email
		deleteUser.Token = deleteToken
		return deleteUser, nil
	}

	if in.GetToken() != "" && in.GetUserId() != "" {
		userID, _ := u.getUserIDByToken(in.GetToken())
		if userID != in.GetUserId() {
			return nil, errTokenIsNotValid
		}

		tx, err := service.db.Conn.Beginx()
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errTransactionCreateError
		}

		data, err := tx.Exec(`UPDATE
        "user"
			SET
				"enabled" = 'f'
			WHERE
				"id" = $1`,
			in.GetUserId(),
		)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToUpdate
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}

		data, err = tx.Exec(`UPDATE
        "user_token"
			SET
				"used" = 't',
				date_used = NOW()
			WHERE
				"token" = $1`,
			in.GetToken(),
		)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToUpdate
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}

		if err := tx.Commit(); err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errTransactionCommitError
		}

		err = service.db.Conn.QueryRow(`SELECT
				"name",
				"email"
			FROM
				"user"
			WHERE
				"id" = $1`,
			in.GetUserId(),
		).Scan(&name, &email)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToScan
		}

		deleteUser.Name = name
		deleteUser.Email = email
		return deleteUser, nil
	}

	return deleteUser, nil
}

// UpdatePassword is ...
func (u *user) UpdatePassword(ctx context.Context, in *pb_user.UpdatePassword_Request) (*pb_user.UpdatePassword_Response, error) {
	if len(in.GetOldPassword()) > 0 {
		// Check for a validity of the old password
		var currentPassword string
		err := service.db.Conn.QueryRow(`SELECT "password" FROM "user" WHERE "id" = $1`, in.GetUserId()).Scan(&currentPassword)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToSelect
		}
		if !crypto.CheckPasswordHash(in.GetOldPassword(), currentPassword) {
			return nil, errPasswordIsNotValid // Old password is not valid
		}
	}

	// We change the old password for a new
	newPassword, err := crypto.HashPassword(in.GetNewPassword())
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errHashIsNotValid // HashPassword failed
	}

	data, err := service.db.Conn.Exec(`UPDATE "user" SET "password" = $1 WHERE "id" = $2`, newPassword, in.GetUserId())
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	// return &pb_user.UpdatePassword_Response{
	//	 Message: "Password update",
	// }, nil
	return &pb_user.UpdatePassword_Response{}, nil
}

// SignIn is ...
func (u *user) SignIn(ctx context.Context, in *pb_user.SignIn_Request) (*pb_user.User_Response, error) {
	var password string
	user := new(pb_user.User_Response)
	user.Email = in.GetEmail()
	err := service.db.Conn.QueryRow(`SELECT
			"id",
			"fio",
			"name",
			"password",
			"enabled",
			"confirmed",
			"role"
		FROM
			"user"
		WHERE
			"email" = $1
			AND "enabled" = 't'
			AND "confirmed" = 't'`,
		in.GetEmail(),
	).Scan(
		&user.UserId,
		&user.Fio,
		&user.Name,
		&password,
		&user.Enabled,
		&user.Confirmed,
		&user.Role,
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		// return nil, errNotFound
		return nil, errFailedToScan
	}

	// Does he have access to the admin panel
	// if user.GetRole() != pb_user.RoleUser_ADMIN && in.GetApp() == pb_user.AppType_admin {
	//	return nil, errors.New(internal.MsgAccessIsDenied)
	// }

	if !crypto.CheckPasswordHash(in.GetPassword(), password) {
		return nil, errPasswordIsNotValid
	}

	return user, nil
}

// ResetPassword is ...
func (u *user) ResetPassword(ctx context.Context, in *pb_user.ResetPassword_Request) (*pb_user.ResetPassword_Response, error) {
	var id, resetToken string
	resetPassword := new(pb_user.ResetPassword_Response)

	// Sending an email with a verification link
	if in.GetEmail() != "" {
		// Check if there is a user with the specified email
		err := service.db.Conn.QueryRow(`SELECT "id" FROM "user" WHERE "email" = $1 AND "enabled" = 't'`, in.GetEmail()).Scan(&id)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToSelect
		}
		if id == "" {
			resetPassword.Message = "Verification email has been sent"
			return resetPassword, nil
		}

		// Checking if a verification token has been sent in the last 24 hours
		resetToken, _ = u.getTokenByUserID(id, "reset")
		if len(resetToken) > 0 {
			resetPassword.Message = "Resend only after 24 hours"
			return resetPassword, nil
		}

		resetToken = uuid.New().String()
		data, err := service.db.Conn.Exec(`INSERT
			INTO "user_token" (
				"token",
				"user_id",
				"date_create",
				"action"
			)
			VALUES
				($1, $2, NOW(), 'reset')`,
			resetToken,
			id,
		)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToAdd
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}

		resetPassword.Message = "Verification email has been sent"
		resetPassword.Token = resetToken
		return resetPassword, nil
	}

	// Checking the validity of a link
	if in.GetToken() != "" && in.GetPassword() == "" {
		if _, err := u.getUserIDByToken(in.GetToken()); err != nil {
			service.log.FromGRPC(err).Send()
			return nil, err
		}

		resetPassword.Message = "Token is valid"
		return resetPassword, nil
	}

	// Saving a new password
	if in.GetToken() != "" && in.GetPassword() != "" {
		id, err := u.getUserIDByToken(in.GetToken())
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, err
		}

		newPassword, err := crypto.HashPassword(in.GetPassword())
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errHashIsNotValid // HashPassword failed
		}

		tx, err := service.db.Conn.Beginx()
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errTransactionCreateError
		}

		tx.MustExec(`UPDATE "user"
			SET
				"password" = $1
			WHERE
				"id" = $2`,
			newPassword,
			id,
		)

		tx.MustExec(`UPDATE "user_token"
			SET
				"used" = 't',
				date_used = NOW()
			WHERE
				"token" = $1`,
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

		resetPassword.Message = "New password saved"
		return resetPassword, nil
	}

	return resetPassword, nil
}

// getUserIDByToken
func (u *user) getUserIDByToken(token string) (string, error) {
	var id string
	err := service.db.Conn.QueryRow(`SELECT
			"user_id" AS "id"
		FROM
			"user_token"
		WHERE
			"token" = $1
			AND "used" = 'f'
			AND "date_create" > NOW() - INTERVAL '24 hour'`,
		token,
	).Scan(&id)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return id, errFailedToScan
	}
	if id == "" {
		return id, errTokenIsNotValid
	}

	return id, nil
}

// getTokenByUserID
func (u *user) getTokenByUserID(userID, action string) (string, error) {
	var token string
	err := service.db.Conn.QueryRow(`SELECT
			"token"
		FROM
			"user_token"
		WHERE
			"user_id" = $1
			AND "used" = 'f'
			AND "action" = $2
			AND "date_create" > NOW() - INTERVAL '24 hour'`,
		userID,
		action,
	).Scan(&token)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return token, errFailedToScan
	}
	if token == "" {
		return token, errTokenIsNotValid
	}

	return token, nil
}
