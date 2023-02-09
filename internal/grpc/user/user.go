package user

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/werbot/werbot/internal/crypto"
	userpb "github.com/werbot/werbot/internal/grpc/user/proto"
)

// ListUsers is lists all users on the system
func (h *Handler) ListUsers(ctx context.Context, in *userpb.ListUsers_Request) (*userpb.ListUsers_Response, error) {
	response := new(userpb.ListUsers_Response)

	sqlSearch := h.DB.SQLAddWhere(in.GetQuery())
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := h.DB.Conn.Query(`SELECT
      "id",
      "login",
      "name",
      "surname",
      "email",
      "enabled",
      "confirmed",
      "last_update",
      "last_update",
      "created",
      "role",
      (SELECT COUNT(*) FROM "project" WHERE "owner_id" = "user"."id") AS "count_project",
      (SELECT COUNT(*) FROM "user_public_key" WHERE "user_id" = "user"."id") AS "count_keys",
      (SELECT COUNT(*) FROM "project" JOIN "server" ON "project"."id"="server"."project_id" WHERE "project"."owner_id"="user"."id") AS "count_servers"
    FROM "user"` + sqlSearch + sqlFooter)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	for rows.Next() {
		var countServers, countProjects, countKeys int32
		var lastUpdate, created pgtype.Timestamp
		user := new(userpb.User_Response)
		userDetail := new(userpb.ListUsers_Response_UserInfo)
		err = rows.Scan(&user.UserId,
			&user.Login,
			&user.Name,
			&user.Surname,
			&user.Email,
			&user.Enabled,
			&user.Confirmed,
			&lastUpdate,
			&created,
			&user.Role,
			&countProjects,
			&countKeys,
			&countServers,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			log.FromGRPC(err).Send()
			return nil, errServerError
		}
		user.LastUpdate = timestamppb.New(lastUpdate.Time)
		user.Created = timestamppb.New(created.Time)

		userDetail.ServersCount = countServers
		userDetail.ProjectsCount = countProjects
		userDetail.KeysCount = countKeys
		userDetail.User = user

		response.Users = append(response.Users, userDetail)
	}
	defer rows.Close()

	// Total count for pagination
	err = h.DB.Conn.QueryRow(`SELECT COUNT (*) FROM "user"` + sqlSearch).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	return response, nil
}

// User is displays user information
func (h *Handler) User(ctx context.Context, in *userpb.User_Request) (*userpb.User_Response, error) {
	var lastUpdate, created pgtype.Timestamp
	response := new(userpb.User_Response)
	response.UserId = in.GetUserId()

	err := h.DB.Conn.QueryRow(`SELECT "login", "name", "surname", "email", "enabled", "confirmed", "role", "last_update", "created"
    FROM "user"
    WHERE "id" = $1`,
		in.GetUserId(),
	).Scan(&response.Login,
		&response.Name,
		&response.Surname,
		&response.Email,
		&response.Enabled,
		&response.Confirmed,
		&response.Role,
		&lastUpdate,
		&created,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	response.LastUpdate = timestamppb.New(lastUpdate.Time)
	response.Created = timestamppb.New(created.Time)

	return response, nil
}

// AddUser is adds a new user
func (h *Handler) AddUser(ctx context.Context, in *userpb.AddUser_Request) (*userpb.AddUser_Response, error) {
	response := new(userpb.AddUser_Response)

	tx, err := h.DB.Conn.Begin()
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errTransactionCreateError
	}

	// Checking if the email address already exists
	err = tx.QueryRow(`SELECT "id" FROM "user" WHERE "email" = $1`,
		in.GetEmail(),
	).Scan(&response.UserId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		log.FromGRPC(err).Send()
		return nil, errServerError
	}
	if response.UserId != "" {
		return nil, errObjectAlreadyExists
	}

	// Adds a new entry to the database
	password, _ := crypto.HashPassword(in.Password)
	err = tx.QueryRow(`INSERT INTO "user" ("login", "name", "surname", "email", "password", "enabled", "confirmed")
    VALUES ($1, $2, $3, $4, $5, $6)
    RETURNING "id"`,
		in.GetLogin(),
		in.GetName(),
		in.GetSurname(),
		in.GetEmail(),
		password,
		in.GetEnabled(),
		in.GetConfirmed(),
	).Scan(&response.UserId)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	if err = tx.Commit(); err != nil {
		log.FromGRPC(err).Send()
		return nil, errTransactionCommitError
	}

	return response, nil
}

// UpdateUser is updates user data
func (h *Handler) UpdateUser(ctx context.Context, in *userpb.UpdateUser_Request) (*userpb.UpdateUser_Response, error) {
	var err error
	var data sql.Result
	response := new(userpb.UpdateUser_Response)

	switch in.GetRequest().(type) {
	case *userpb.UpdateUser_Request_Info:
		data, err = h.DB.Conn.Exec(`UPDATE "user"
    SET "login" = $1,
      "email" = $2,
      "name" = $3,
      "surname" = $4,
      "last_update" = NEW()
    WHERE "id" = $5`,
			in.GetInfo().GetLogin(),
			in.GetInfo().GetEmail(),
			in.GetInfo().GetName(),
			in.GetInfo().GetSurname(),
			in.GetUserId(),
		)

	case *userpb.UpdateUser_Request_Confirmed:
		data, err = h.DB.Conn.Exec(`UPDATE "user" SET "confirmed" = $1, "last_update" = NEW() WHERE "id" = $2`,
			in.GetConfirmed(),
			in.GetUserId(),
		)

	case *userpb.UpdateUser_Request_Enabled:
		data, err = h.DB.Conn.Exec(`UPDATE "user" SET "enabled" = $1, "last_update" = NEW() WHERE "id" = $2`,
			in.GetEnabled(),
			in.GetUserId(),
		)

	default:
		return nil, errBadRequest
	}

	if err != nil {
		log.FromGRPC(err).Send()
		return nil, err
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// DeleteUser is ...
func (h *Handler) DeleteUser(ctx context.Context, in *userpb.DeleteUser_Request) (*userpb.DeleteUser_Response, error) {
	var login, passwordHash, email string
	response := new(userpb.DeleteUser_Response)

	switch in.GetRequest().(type) {
	case *userpb.DeleteUser_Request_Password:
		tx, err := h.DB.Conn.Begin()
		if err != nil {
			log.FromGRPC(err).Send()
			return nil, errTransactionCreateError
		}

		err = tx.QueryRow(`SELECT "login", "password", "email" FROM "user" WHERE "id" = $1`,
			in.GetUserId(),
		).Scan(&login,
			&passwordHash,
			&email,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			log.FromGRPC(err).Send()
			return nil, errServerError
		}
		if !crypto.CheckPasswordHash(in.GetPassword(), passwordHash) {
			return nil, errPasswordIsNotValid
		}

		// Checking if a verification token has been sent in the last 24 hours
		deleteToken, _ := TokenByUserID(h, in.GetUserId(), "delete")
		if len(deleteToken) > 0 {
			response.Login = login
			response.Email = email
			response.Token = deleteToken
			return response, nil
		}

		deleteToken = uuid.New().String()
		data, err := tx.Exec(`INSERT INTO "user_token" ("token", "user_id", "action") VALUES ($1, $2, 'delete')`,
			deleteToken,
			in.GetUserId())
		if err != nil {
			log.FromGRPC(err).Send()
			return nil, err // Create delete token failed
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}

		if err := tx.Commit(); err != nil {
			log.FromGRPC(err).Send()
			return nil, errTransactionCommitError
		}

		response.Email = email
		response.Token = deleteToken
		return response, nil

	case *userpb.DeleteUser_Request_Token:
		userID, _ := UserIDByToken(h, in.GetToken())
		if userID != in.GetUserId() {
			return nil, errTokenIsNotValid
		}

		tx, err := h.DB.Conn.Begin()
		if err != nil {
			log.FromGRPC(err).Send()
			return nil, errTransactionCreateError
		}

		data, err := tx.Exec(`UPDATE "user" SET "enabled" = 'f', "last_update" = NEW() WHERE "id" = $1`, in.GetUserId())
		if err != nil {
			log.FromGRPC(err).Send()
			return nil, errFailedToUpdate
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}

		data, err = tx.Exec(`UPDATE "user_token" SET "used" = 't', "date_used" = NOW() WHERE "token" = $1`, in.GetToken())
		if err != nil {
			log.FromGRPC(err).Send()
			return nil, errFailedToUpdate
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}

		err = tx.QueryRow(`SELECT "login", "email" FROM "user" WHERE "id" = $1`,
			in.GetUserId(),
		).Scan(&login,
			&email,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			log.FromGRPC(err).Send()
			return nil, errServerError
		}

		if err := tx.Commit(); err != nil {
			log.FromGRPC(err).Send()
			return nil, errTransactionCommitError
		}

		response.Login = login
		response.Email = email
		return response, nil
	}

	return nil, errBadRequest
}

// UpdatePassword is ...
func (h *Handler) UpdatePassword(ctx context.Context, in *userpb.UpdatePassword_Request) (*userpb.UpdatePassword_Response, error) {
	response := new(userpb.UpdatePassword_Response)

	if len(in.GetOldPassword()) > 0 {
		// Check for a validity of the old password
		var currentPassword string
		err := h.DB.Conn.QueryRow(`SELECT "password" FROM "user" WHERE "id" = $1`,
			in.GetUserId(),
		).Scan(&currentPassword)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			log.FromGRPC(err).Send()
			return nil, errServerError
		}
		if !crypto.CheckPasswordHash(in.GetOldPassword(), currentPassword) {
			return nil, errPasswordIsNotValid // Old password is not valid
		}
	}

	// We change the old password for a new
	newPassword, err := crypto.HashPassword(in.GetNewPassword())
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errHashIsNotValid // HashPassword failed
	}

	data, err := h.DB.Conn.Exec(`UPDATE "user" SET "password" = $1, "last_update" = NEW() WHERE "id" = $2`, newPassword, in.GetUserId())
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// TokenByUserID is ...
func TokenByUserID(h *Handler, userID, action string) (string, error) {
	var token string
	err := h.DB.Conn.QueryRow(`SELECT "token"
		FROM "user_token"
		WHERE "user_id" = $1
			AND "used" = 'f'
			AND "action" = $2
			AND "created" > NOW() - INTERVAL '24 hour'`,
		userID,
		action,
	).Scan(&token)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", errNotFound
		}
		log.FromGRPC(err).Send()
		return "", errServerError
	}
	if token == "" {
		return token, errTokenIsNotValid
	}

	return token, nil
}

// UserIDByToken is ...
func UserIDByToken(h *Handler, token string) (string, error) {
	var id string
	err := h.DB.Conn.QueryRow(`SELECT "user_id" AS "id"
		FROM "user_token"
		WHERE "token" = $1
			AND "used" = 'f'
			AND "created" > NOW() - INTERVAL '24 hour'`,
		token,
	).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", errNotFound
		}
		log.FromGRPC(err).Send()
		return "", errServerError
	}
	if id == "" {
		return id, errTokenIsNotValid
	}

	return id, nil
}