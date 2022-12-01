package grpc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/crypto"

	pb_user "github.com/werbot/werbot/internal/grpc/proto/user"
)

type user struct {
	pb_user.UnimplementedUserHandlersServer
}

// ListUsers is lists all users on the system
func (u *user) ListUsers(ctx context.Context, in *pb_user.ListUsers_Request) (*pb_user.ListUsers_Response, error) {
	sqlSearch := db.SQLAddWhere(in.GetQuery())
	sqlFooter := db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())

	rows, err := db.Conn.Query(`SELECT
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
		return nil, errors.New("Failed show all users")
	}

	users := []*pb_user.ListUsers_Response_UserInfo{}
	for rows.Next() {
		user := pb_user.GetUser_Response{}
		var lastActive, registerDate pgtype.Timestamp
		var countServers, countProjects, countKeys int32

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
			return nil, errors.New("Unable to parse response from database")
		}

		user.LastActive = timestamppb.New(lastActive.Time)
		user.RegisterDate = timestamppb.New(registerDate.Time)

		users = append(users, &pb_user.ListUsers_Response_UserInfo{
			ServersCount:  countServers,
			ProjectsCount: countProjects,
			KeysCount:     countKeys,
			User:          &user,
		})
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	err = db.Conn.QueryRow(`SELECT COUNT (*) FROM "user"` + sqlSearch).Scan(&total)
	if err != nil {
		return nil, errors.New("Query Execution Problem")
	}

	return &pb_user.ListUsers_Response{
		Total: total,
		Users: users,
	}, nil
}

// GetUser is displays user information
func (u *user) GetUser(ctx context.Context, in *pb_user.GetUser_Request) (*pb_user.GetUser_Response, error) {
	user := pb_user.GetUser_Response{
		UserId: in.GetUserId(),
	}

	err := db.Conn.QueryRow(`SELECT
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
		return nil, errors.New("Failed parse user information")
	}

	return &user, nil
}

// CreateUser is adds a new user
func (u *user) CreateUser(ctx context.Context, in *pb_user.CreateUser_Request) (*pb_user.CreateUser_Response, error) {
	tx, err := db.Conn.Beginx()
	if err != nil {
		return nil, errors.New("Error creating transaction")
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
		return nil, errors.New("Query Execution Problem")
	}
	if id != "" {
		return nil, errors.New("User with this email is already registered")
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
		return nil, errors.New("Problem adding new user")
	}

	if err = tx.Commit(); err != nil {
		return nil, errors.New("Transaction commit error")
	}

	return &pb_user.CreateUser_Response{
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
	query := fmt.Sprintf(`UPDATE "user" 
		SET 
			%s 
		WHERE 
			"id" = $%v`,
		strings.Join(qParts, ", "),
		cnt,
	)
	_, err := db.Conn.Exec(query, args...)
	if err != nil {
		return nil, errors.New("Failed to update user data")
	}

	return &pb_user.UpdateUser_Response{}, nil
}

// DeleteUser is ...
func (u *user) DeleteUser(ctx context.Context, in *pb_user.DeleteUser_Request) (*pb_user.DeleteUser_Response, error) {
	var name, passwordHash, email, deleteToken string

	if in.GetPassword() != "" && in.GetUserId() != "" {
		err := db.Conn.QueryRow(`SELECT 
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
			return nil, errors.New("Query Execution Problem")
		}
		if !crypto.CheckPasswordHash(in.GetPassword(), passwordHash) {
			return nil, errors.New("Password is not valid")
		}

		// Checking if a verification token has been sent in the last 24 hours
		deleteToken, _ = u.getTokenByUserID(in.GetUserId(), "delete")
		if len(deleteToken) > 0 {
			return &pb_user.DeleteUser_Response{
				Name:  name,
				Email: email,
				Token: deleteToken,
			}, nil
		}

		deleteToken = uuid.New().String()
		_, err = db.Conn.Exec(`INSERT 
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
			return nil, errors.New("Create delete token failed")
		}

		return &pb_user.DeleteUser_Response{
			Email: email,
			Token: deleteToken,
		}, nil
	}

	if in.GetToken() != "" && in.GetUserId() != "" {
		userID, _ := u.getUserIDByToken(in.GetToken())
		if userID != in.GetUserId() {
			return nil, errors.New("Account deletion token not recognized")
		}

		tx := db.Conn.MustBegin()
		tx.MustExec(`UPDATE "user" 
			SET 
				"enabled" = 'f' 
			WHERE 
				"id" = $1`,
			in.GetUserId(),
		)

		tx.MustExec(`UPDATE "user_token" 
			SET 
				"used" = 't', 
				date_used = NOW() 
			WHERE 
				"token" = $1`,
			in.GetToken(),
		)
		if err := tx.Commit(); err != nil {
			return nil, errors.New("Account deletion failed")
		}

		err := db.Conn.QueryRow(`SELECT 
				"name", 
				"email" 
			FROM 
				"user" 
			WHERE 
				"id" = $1`,
			in.GetUserId(),
		).Scan(&name, &email)
		if err != nil {
			return nil, errors.New("Query Execution Problem")
		}

		return &pb_user.DeleteUser_Response{
			Name:  name,
			Email: email,
		}, nil
	}

	return &pb_user.DeleteUser_Response{}, nil
}

// UpdatePassword is ...
func (u *user) UpdatePassword(ctx context.Context, in *pb_user.UpdatePassword_Request) (*pb_user.UpdatePassword_Response, error) {
	if len(in.GetOldPassword()) > 0 {
		// проверяем на валидность старый пароль
		var currentPassword string
		err := db.Conn.QueryRow(`SELECT "password" FROM "user" WHERE "id" = $1`, in.GetUserId()).Scan(&currentPassword)
		if err != nil {
			return nil, errors.New("Query Execution Problem")
		}
		if !crypto.CheckPasswordHash(in.GetOldPassword(), currentPassword) {
			return nil, errors.New("Old password is not valid")
		}
	}

	// изменяем старый пароль на новый
	newPassword, err := crypto.HashPassword(in.GetNewPassword())
	if err != nil {
		return nil, errors.New("HashPassword failed")
	}

	_, err = db.Conn.Exec(`UPDATE "user" SET "password" = $1 WHERE "id" = $2`, newPassword, in.GetUserId())
	if err != nil {
		return nil, errors.New("UpdateUser failed")
	}

	return &pb_user.UpdatePassword_Response{
		Message: "Password update",
	}, nil
}

// SignIn is ...
func (u *user) SignIn(ctx context.Context, in *pb_user.SignIn_Request) (*pb_user.GetUser_Response, error) {
	user := pb_user.GetUser_Response{
		Email: in.GetEmail(),
	}

	var password string
	err := db.Conn.QueryRow(`SELECT 
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
		return nil, errors.New(internal.ErrNotFound)
	}

	// Does he have access to the admin panel
	//if user.GetRole() != pb_user.RoleUser_ADMIN && in.GetApp() == pb_user.AppType_admin {
	//	return nil, errors.New(internal.MsgAccessIsDenied)
	//}

	if !crypto.CheckPasswordHash(in.GetPassword(), password) {
		return nil, errors.New(internal.ErrInvalidPassword)
	}

	return &user, nil
}

// ResetPassword is ...
func (u *user) ResetPassword(ctx context.Context, in *pb_user.ResetPassword_Request) (*pb_user.ResetPassword_Response, error) {
	var id, resetToken string

	// Sending an email with a verification link
	if in.GetEmail() != "" {
		// Check if there is a user with the specified email
		err := db.Conn.QueryRow(`SELECT "id" FROM "user" WHERE "email" = $1 AND "enabled" = 't'`, in.GetEmail()).Scan(&id)
		if err != nil {
			return nil, errors.New("Query Execution Problem")
		}
		if id == "" {
			return &pb_user.ResetPassword_Response{
				Message: "Your verification email has been sent",
			}, nil
		}

		// Checking if a verification token has been sent in the last 24 hours
		resetToken, _ = u.getTokenByUserID(id, "reset")
		if len(resetToken) > 0 {
			return &pb_user.ResetPassword_Response{
				Message: "Resend only after 24 hours",
			}, nil
		}

		resetToken = uuid.New().String()
		_, err = db.Conn.Exec(`INSERT 
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
			return nil, errors.New("Create reset token failed")
		}

		return &pb_user.ResetPassword_Response{
			Message: "Your verification email has been sent",
			Token:   resetToken,
		}, nil
	}

	// Checking the validity of a link
	if in.GetToken() != "" && in.GetPassword() == "" {
		if _, err := u.getUserIDByToken(in.GetToken()); err != nil {
			return nil, err
		}

		return &pb_user.ResetPassword_Response{
			Message: "Token is valid",
		}, nil
	}

	// Saving a new password
	if in.GetToken() != "" && in.GetPassword() != "" {
		id, err := u.getUserIDByToken(in.GetToken())
		if err != nil {
			return nil, err
		}

		newPassword, err := crypto.HashPassword(in.GetPassword())
		if err != nil {
			return nil, errors.New("HashPassword failed")
		}

		tx := db.Conn.MustBegin()
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
			return nil, errors.New("There was a problem updating")
		}

		/*
		   if _, err = db.Conn.Exec(`UPDATE "user" SET "password" = $1 WHERE "id" = $2`, newPassword, id); err != nil {
		     return nil, errors.New("There was a problem updating")
		   }

		   if _, err = db.Conn.Exec(`UPDATE "user_token" SET "used" = 't' WHERE "token" = $1`, in.GetToken()); err != nil {
		     return nil, errors.New("There was a problem updating")
		   }
		*/

		return &pb_user.ResetPassword_Response{
			Message: "New password saved",
		}, nil
	}

	return &pb_user.ResetPassword_Response{}, nil
}

// getUserIDByToken
func (u *user) getUserIDByToken(token string) (string, error) {
	var id string
	err := db.Conn.QueryRow(`SELECT 
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
		return id, errors.New("Query Execution Problem")
	}
	if id == "" {
		return id, errors.New("Token is invalid")
	}

	return id, nil
}

// getTokenByUserID
func (u *user) getTokenByUserID(userID, action string) (string, error) {
	var token string
	err := db.Conn.QueryRow(`SELECT 
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
		return token, errors.New("Query Execution Problem")
	}
	if token == "" {
		return token, errors.New("Token is invalid")
	}

	return token, nil
}
