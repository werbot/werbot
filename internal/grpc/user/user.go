package user

import (
  "context"
  "database/sql"

  "github.com/google/uuid"
  "github.com/jackc/pgx/v5/pgtype"
  "google.golang.org/grpc/codes"
  "google.golang.org/protobuf/types/known/timestamppb"

  "github.com/werbot/werbot/internal/crypto"
  userpb "github.com/werbot/werbot/internal/grpc/user/proto"
  "github.com/werbot/werbot/internal/trace"
)

// ListUsers is lists all users on the system
func (h *Handler) ListUsers(ctx context.Context, in *userpb.ListUsers_Request) (*userpb.ListUsers_Response, error) {
  response := new(userpb.ListUsers_Response)

  sqlSearch := h.DB.SQLAddWhere(in.GetQuery())
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
      "last_update",
      "last_update",
      "created",
      "role",
      (
        SELECT
          COUNT(*)
        FROM
          "project"
        WHERE
          "owner_id" = "user"."id"
      ) AS "count_project",
      (
        SELECT
          COUNT(*)
        FROM
          "user_public_key"
        WHERE
          "user_id" = "user"."id"
      ) AS "count_keys",
      (
        SELECT
          COUNT(*)
        FROM
          "project"
          JOIN "server" ON "project"."id" = "server"."project_id"
        WHERE
          "project"."owner_id" = "user"."id"
      ) AS "count_servers"
    FROM
      "user"
  `+sqlSearch+sqlFooter)
  if err != nil {
    return nil, trace.ErrorAborted(err, log)
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
      return nil, trace.ErrorAborted(err, log)
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
  err = h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      COUNT(*)
    FROM
      "user"
  `+sqlSearch).Scan(&response.Total)
  if err != nil && err != sql.ErrNoRows {
    return nil, trace.ErrorAborted(err, log)
  }

  return response, nil
}

// User is displays user information
func (h *Handler) User(ctx context.Context, in *userpb.User_Request) (*userpb.User_Response, error) {
  var lastUpdate, created pgtype.Timestamp
  response := new(userpb.User_Response)
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
      "last_update",
      "created"
    FROM
      "user"
    WHERE
      "id" = $1
  `, in.GetUserId(),
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
    return nil, trace.ErrorAborted(err, log)
  }

  response.LastUpdate = timestamppb.New(lastUpdate.Time)
  response.Created = timestamppb.New(created.Time)

  return response, nil
}

// AddUser is adds a new user
func (h *Handler) AddUser(ctx context.Context, in *userpb.AddUser_Request) (*userpb.AddUser_Response, error) {
  response := new(userpb.AddUser_Response)

  tx, err := h.DB.Conn.BeginTx(ctx, nil)
  if err != nil {
    return nil, trace.ErrorAborted(err, log, trace.MsgTransactionCreateError)
  }
  defer tx.Rollback()

  // Checking if the email address already exists
  err = tx.QueryRowContext(ctx, `
    SELECT
      "id"
    FROM
      "user"
    WHERE
      "email" = $1
  `, in.GetEmail(),
  ).Scan(&response.UserId)
  if err != nil && err != sql.ErrNoRows {
    return nil, trace.ErrorAborted(err, log)
  }

  if response.UserId != "" {
    return nil, trace.Error(codes.AlreadyExists)
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
    VALUES
      ($1, $2, $3, $4, $5, $6)
    RETURNING
      "id"
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
    return nil, trace.ErrorAborted(err, log, trace.MsgFailedToAdd)
  }

  if err := tx.Commit(); err != nil {
    return nil, trace.ErrorAborted(err, log, trace.MsgTransactionCommitError)
  }

  return response, nil
}

// UpdateUser is updates user data
func (h *Handler) UpdateUser(ctx context.Context, in *userpb.UpdateUser_Request) (*userpb.UpdateUser_Response, error) {
  var err error
  response := new(userpb.UpdateUser_Response)

  switch in.GetRequest().(type) {
  case *userpb.UpdateUser_Request_Info:
    _, err = h.DB.Conn.ExecContext(ctx, `
      UPDATE "user"
      SET
        "login" = $1,
        "email" = $2,
        "name" = $3,
        "surname" = $4,
        "last_update" = NOW()
      WHERE
        "id" = $5
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
      SET
        "confirmed" = $1,
        "last_update" = NOW()
      WHERE
        "id" = $2
    `, in.GetConfirmed(), in.GetUserId())
  case *userpb.UpdateUser_Request_Enabled:
    _, err = h.DB.Conn.ExecContext(ctx, `
      UPDATE "user"
      SET
        "enabled" = $1,
        "last_update" = NOW()
      WHERE
        "id" = $2
    `, in.GetEnabled(), in.GetUserId())
  default:
    return nil, trace.Error(codes.InvalidArgument)
  }

  if err != nil {
    return nil, trace.ErrorAborted(err, log, trace.MsgFailedToUpdate)
  }

  return response, nil
}

// DeleteUser is ...
func (h *Handler) DeleteUser(ctx context.Context, in *userpb.DeleteUser_Request) (*userpb.DeleteUser_Response, error) {
  var login, passwordHash, email string
  response := new(userpb.DeleteUser_Response)

  switch in.GetRequest().(type) {
  case *userpb.DeleteUser_Request_Password:
    tx, err := h.DB.Conn.BeginTx(ctx, nil)
    if err != nil {
      return nil, trace.ErrorAborted(err, log, trace.MsgTransactionCreateError)
    }
    defer tx.Rollback()

    err = tx.QueryRowContext(ctx, `
      SELECT
        "login",
        "password",
        "email"
      FROM
        "user"
      WHERE
        "id" = $1
    `, in.GetUserId(),
    ).Scan(&login,
      &passwordHash,
      &email,
    )
    if err != nil {
      return nil, trace.ErrorAborted(err, log)
    }

    if !crypto.CheckPasswordHash(in.GetPassword(), passwordHash) {
      return nil, trace.Error(codes.InvalidArgument, trace.MsgPasswordIsNotValid)
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
      INSERT INTO
        "user_token" ("token", "user_id", "action")
      VALUES
        ($1, $2, 'delete')
    `, deleteToken, in.GetUserId())
    if err != nil {
      return nil, trace.ErrorAborted(err, log, trace.MsgFailedToAdd)
    }

    if err := tx.Commit(); err != nil {
      return nil, trace.ErrorAborted(err, log, trace.MsgTransactionCommitError)
    }

    response.Email = email
    response.Token = deleteToken
    return response, nil

  case *userpb.DeleteUser_Request_Token:
    userID, _ := UserIDByToken(ctx, h, in.GetToken())
    if userID != in.GetUserId() {
      return nil, trace.Error(codes.InvalidArgument, trace.MsgInviteIsInvalid)
    }

    tx, err := h.DB.Conn.BeginTx(ctx, nil)
    if err != nil {
      return nil, trace.ErrorAborted(err, log, trace.MsgTransactionCreateError)
    }
    defer tx.Rollback()

    _, err = tx.ExecContext(ctx, `
      UPDATE "user"
      SET
        "enabled" = 'f',
        "last_update" = NOW()
      WHERE
        "id" = $1
    `, in.GetUserId())
    if err != nil {
      return nil, trace.ErrorAborted(err, log, trace.MsgFailedToUpdate)
    }

    _, err = tx.ExecContext(ctx, `
      UPDATE "user_token"
      SET
        "used" = 't',
        "date_used" = NOW()
      WHERE
        "token" = $1
    `, in.GetToken())
    if err != nil {
      return nil, trace.ErrorAborted(err, log, trace.MsgFailedToUpdate)
    }

    err = tx.QueryRowContext(ctx, `
      SELECT
        "login",
        "email"
      FROM
        "user"
      WHERE
        "id" = $1
    `, in.GetUserId()).Scan(&login,
      &email,
    )
    if err != nil {
      return nil, trace.ErrorAborted(err, log)
    }

    if err := tx.Commit(); err != nil {
      return nil, trace.ErrorAborted(err, log, trace.MsgTransactionCommitError)
    }

    response.Login = login
    response.Email = email
    return response, nil
  }

  return nil, trace.Error(codes.InvalidArgument)
}

// UpdatePassword is ...
func (h *Handler) UpdatePassword(ctx context.Context, in *userpb.UpdatePassword_Request) (*userpb.UpdatePassword_Response, error) {
  response := new(userpb.UpdatePassword_Response)

  if len(in.GetOldPassword()) > 0 {
    // Check for a validity of the old password
    var currentPassword string
    err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT
        "password"
      FROM
        "user"
      WHERE
        "id" = $1
    `, in.GetUserId(),
    ).Scan(&currentPassword)
    if err != nil {
      return nil, trace.ErrorAborted(err, log)
    }

    if !crypto.CheckPasswordHash(in.GetOldPassword(), currentPassword) {
      return nil, trace.Error(codes.InvalidArgument, trace.MsgPasswordIsNotValid)
    }
  }

  // We change the old password for a new
  newPassword, err := crypto.HashPassword(in.GetNewPassword())
  if err != nil {
    trace.Error(codes.InvalidArgument, trace.MsgPasswordIsNotValid)
  }

  _, err = h.DB.Conn.ExecContext(ctx, `
    UPDATE "user"
    SET
      "password" = $1,
      "last_update" = NOW()
    WHERE
      "id" = $2
  `, newPassword, in.GetUserId())
  if err != nil {
    return nil, trace.ErrorAborted(err, log, trace.MsgFailedToUpdate)
  }

  return response, nil
}

// TokenByUserID is ...
func TokenByUserID(ctx context.Context, h *Handler, userID, action string) (string, error) {
  var token string
  err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "token"
    FROM
      "user_token"
    WHERE
      "user_id" = $1
      AND "used" = 'f'
      AND "action" = $2
      AND "created" > NOW() - INTERVAL '24 hour'
  `, userID, action,
  ).Scan(&token)
  if err != nil {
    if err == sql.ErrNoRows {
      return "", trace.Error(codes.NotFound, trace.MsgInviteIsInvalid)
    }
    return "", trace.ErrorAborted(err, log)
  }

  if token == "" {
    return token, trace.Error(codes.InvalidArgument, trace.MsgInviteIsInvalid)
  }

  return token, nil
}

// UserIDByToken is ...
func UserIDByToken(ctx context.Context, h *Handler, token string) (string, error) {
  var id string
  err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "user_id" AS "id"
    FROM
      "user_token"
    WHERE
      "token" = $1
      AND "used" = 'f'
      AND "created" > NOW() - INTERVAL '24 hour'
  `, token,
  ).Scan(&id)
  if err != nil {
    if err == sql.ErrNoRows {
      return "", trace.Error(codes.NotFound, trace.MsgInviteIsInvalid)
    }
    return "", trace.ErrorAborted(err, log)
  }

  if id == "" {
    return id, trace.Error(codes.InvalidArgument, trace.MsgInviteIsInvalid)
  }

  return id, nil
}
