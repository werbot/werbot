package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/crypto"
	keypb "github.com/werbot/werbot/internal/grpc/key/proto"
	serverpb "github.com/werbot/werbot/internal/grpc/server/proto"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/strutil"
)

// TODO: a method for updating Host FingerPrint server
// TODO: When updating the IP address of the server, you need to update HostKey !!!!

// ListServers is displays a list of available servers
func (h *Handler) ListServers(ctx context.Context, in *serverpb.ListServers_Request) (*serverpb.ListServers_Response, error) {
	// TODO: checking the permitted servers available for display
	//  log.Info().Msgf("Expired: %v", license.L.GetCustomer())

	response := &serverpb.ListServers_Response{}
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())

	if in.GetLogin() != "" {
		loginArray := strutil.SplitNTrimmed(in.GetLogin(), "_", 3)

		nameLen := len(loginArray)
		query := `
      SELECT DISTINCT
        ON ("server"."id") "server"."id",
        "server"."access_id",
        "server"."port",
        "server"."address",
        "server"."token",
        "server_access"."login",
        "server"."title",
        "server"."audit",
        "server"."online",
        "server_access"."auth",
        "server"."scheme",
        "server_host_key"."host_key",
        "server_member"."id" AS "account_id",
        "project_member"."project_id",
        "project"."login" AS "project_login",
      FROM
        "user"
        JOIN "project_member" ON "user"."id" = "project_member"."user_id"
        JOIN "project" ON "project"."id" = "project_member"."project_id"
        JOIN "server" ON "project"."id" = "server"."project_id"
        JOIN "server_host_key" ON "server_host_key"."server_id" = "server"."id"
        JOIN "server_member" ON "server_member"."server_id" = "server"."id"
        AND "server_member"."member_id" = "project_member"."id"
        INNER JOIN "server_access" ON "server"."access_id" = "server_access"."id"
      WHERE
        "user"."login" = $1
        AND "server_member"."active" = TRUE
        AND "server"."active" = TRUE
    `

		switch nameLen {
		case 1:
			rows, err := h.DB.Conn.QueryContext(ctx, query, loginArray[0])
			if err != nil {
				return nil, trace.Error(err, log, nil)
			}

			for rows.Next() {
				server := &serverpb.Server_Response{}
				err = rows.Scan(&server.ServerId,
					&server.AccessId,
					&server.Port,
					&server.Address,
					&server.Token,
					&server.Login,
					&server.Title,
					&server.Audit,
					&server.Online,
					&server.Auth,
					&server.Scheme,
					&server.HostKey,
					&server.AccountId,
					&server.ProjectId,
					&server.ProjectLogin,
				)
				if err != nil {
					return nil, trace.Error(err, log, nil)
				}

				response.Servers = append(response.Servers, server)
			}
			defer rows.Close()

		case 2:
			rows, err := h.DB.Conn.QueryContext(ctx, query+`
        AND "project"."login" = $2`, loginArray[0], loginArray[1])
			if err != nil {
				return nil, trace.Error(err, log, nil)
			}

			for rows.Next() {
				server := &serverpb.Server_Response{}
				err = rows.Scan(&server.ServerId,
					&server.AccessId,
					&server.Port,
					&server.Address,
					&server.Token,
					&server.Login,
					&server.Title,
					&server.Audit,
					&server.Online,
					&server.Auth,
					&server.Scheme,
					&server.HostKey,
					&server.AccountId,
					&server.ProjectId,
					&server.ProjectLogin,
				)
				if err != nil {
					return nil, trace.Error(err, log, nil)
				}

				response.Servers = append(response.Servers, server)
			}
			defer rows.Close()

		case 3:
			rows, err := h.DB.Conn.QueryContext(ctx, query+`
        AND "project"."login" = $2
        AND "token" = $3
        AND "project_member"."role" = 'user'`, loginArray[0], loginArray[1], loginArray[2])
			if err != nil {
				return nil, trace.Error(err, log, nil)
			}

			for rows.Next() {
				server := &serverpb.Server_Response{}
				err = rows.Scan(&server.ServerId,
					&server.AccessId,
					&server.Port,
					&server.Address,
					&server.Token,
					&server.Login,
					&server.Title,
					&server.Audit,
					&server.Online,
					&server.Auth,
					&server.Scheme,
					&server.HostKey,
					&server.AccountId,
					&server.ProjectId,
					&server.ProjectLogin,
				)
				if err != nil {
					return nil, trace.Error(err, log, nil)
				}

				response.Servers = append(response.Servers, server)
			}
			defer rows.Close()
		}

		response.Total = int32(len(response.Servers))
		if response.Total == 0 {
			errGRPC := status.Error(codes.NotFound, "")
			return nil, trace.Error(errGRPC, log, nil)
		}

		return response, nil
	}

	if in.GetProjectId() != "" && in.GetUserId() != "" {
		rows, err := h.DB.Conn.QueryContext(ctx, `
      SELECT DISTINCT
        ON ("server"."id") "server"."id",
        "server"."access_id",
        "server"."address",
        "server"."port",
        "server"."token",
        "server_access"."login",
        "server"."title",
        "server"."audit",
        "server"."online",
        "server_access"."auth",
        "server"."active",
        "server"."scheme",
        "server"."description",
        (
          SELECT COUNT(*)
          FROM "server_member"
          WHERE "server_id" = "server"."id"
        ) AS "count_members"
      FROM
        "server"
        INNER JOIN "project" ON "server"."project_id" = "project"."id"
        INNER JOIN "server_access" ON "server"."access_id" = "server_access"."id"
      WHERE
        "server"."project_id" = $1
        AND "project"."owner_id" = $2
    `+sqlFooter, in.GetProjectId(), in.GetUserId(),
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		for rows.Next() {
			server := &serverpb.Server_Response{}
			err = rows.Scan(&server.ServerId,
				&server.AccessId,
				&server.Address,
				&server.Port,
				&server.Token,
				&server.Login,
				&server.Title,
				&server.Audit,
				&server.Online,
				&server.Auth,
				&server.Active,
				&server.Scheme,
				&server.Description,
				&server.CountMembers,
			)
			if err != nil {
				return nil, trace.Error(err, log, nil)
			}

			response.Servers = append(response.Servers, server)
		}
		defer rows.Close()

		// Total count
		err = h.DB.Conn.QueryRowContext(ctx, `
      SELECT COUNT(*)
      FROM
        "server"
        INNER JOIN "project" ON "server"."project_id" = "project"."id"
      WHERE
        "server"."project_id" = $1
        AND "project"."owner_id" = $2
    `, in.GetProjectId(), in.GetUserId(),
		).Scan(&response.Total)
		if err != nil && err != sql.ErrNoRows {
			return nil, trace.Error(err, log, nil)
		}

		return response, nil
	}

	return response, nil
}

// Server is displays data on the server
func (h *Handler) Server(ctx context.Context, in *serverpb.Server_Request) (*serverpb.Server_Response, error) {
	var description pgtype.Text
	response := &serverpb.Server_Response{}
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "server"."access_id",
      "server"."address",
      "server"."port",
      "server"."token",
      "server_access"."login",
      "server"."description",
      "server"."title",
      "server"."active",
      "server"."audit",
      "server"."online",
      "server_access"."auth",
      "server"."scheme"
    FROM
      "server"
      INNER JOIN "server_access" ON "server"."access_id" = "server_access"."id"
      INNER JOIN "project" ON "server"."project_id" = "project"."id"
    WHERE
      "server"."id" = $1
      AND "project"."id" = $2
      AND "project"."owner_id" = $3
  `, in.GetServerId(), in.GetProjectId(), in.GetUserId(),
	).Scan(&response.AccountId,
		&response.Address,
		&response.Port,
		&response.Token,
		&response.Login,
		&description,
		&response.Title,
		&response.Active,
		&response.Audit,
		&response.Online,
		&response.Auth,
		&response.Scheme,
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	response.Description = description.String
	return response, nil
}

// AddServer is ...
func (h *Handler) AddServer(ctx context.Context, in *serverpb.AddServer_Request) (*serverpb.AddServer_Response, error) {
	var err error
	response := &serverpb.AddServer_Response{}

	serverToken := crypto.NewPassword(6, false)
	serverTitle := in.GetTitle()
	if in.Title == "" {
		serverTitle = fmt.Sprintf("server #%s", serverToken)
	}

	tx, err := h.DB.Conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCreateError)
	}
	defer tx.Rollback()

	var exists bool
	err = tx.QueryRowContext(ctx, `
    SELECT
      EXISTS (
        SELECT 1
        FROM "project"
        WHERE
          "id" = $1
          AND "owner_id" = $2
      )
  `, in.GetProjectId(), in.GetUserId(),
	).Scan(&exists)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	if !exists {
		errGRPC := status.Error(codes.NotFound, "")
		return nil, trace.Error(errGRPC, log, nil)
	}

	err = tx.QueryRowContext(ctx, `
    INSERT INTO
      "server" (
        "project_id",
        "address",
        "port",
        "token",
        "description",
        "title",
        "active",
        "audit",
        "scheme"
      )
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::INT)
    RETURNING "id"
  `,
		in.GetProjectId(),
		in.GetAddress(),
		in.GetPort(),
		serverToken,
		in.GetDescription(),
		serverTitle,
		in.GetActive(),
		in.GetAudit(),
		in.GetScheme().Number(),
	).Scan(&response.ServerId)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
	}

	// + record access
	var accessID, sqlAccess string
	switch in.GetAccess().(type) {
	case *serverpb.AddServer_Request_Password:
		sqlAccess, err = sanitize.SQL(`
      INSERT INTO
        "server_access" (
          "server_id",
          "auth",
          "login",
          "password"
        )
      VALUES ($1, '1', $2, $3)
      RETURNING "id"
    `, response.GetServerId(), in.GetLogin(), in.GetPassword(),
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

	case *serverpb.AddServer_Request_Key:
		key := &keypb.GenerateSSHKey_Key{}
		keyTmp, err := h.Redis.Client.Get(ctx, fmt.Sprintf("tmp_key_ssh:%s", in.GetKey())).Result()
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if err := json.Unmarshal([]byte(keyTmp), key); err != nil {
			return nil, trace.Error(err, log, trace.MsgStructureIsBroken)
		}

		h.Redis.Client.Del(ctx, fmt.Sprintf("tmp_key_ssh:%s", in.GetKey()))

		keyAccess := map[string]string{"public": "" + key.GetPublic() + "", "private": "" + key.GetPrivate() + "", "password": "" + key.GetPassword() + ""}
		keyAccessJSON, _ := json.Marshal(keyAccess)
		sqlAccess, err = sanitize.SQL(`
      INSERT INTO "server_access" ("server_id", "auth", "login", "key")
      VALUES ($1, '2', $2, $3)
      RETURNING "id"
    `, response.GetServerId(), in.GetLogin(), string(keyAccessJSON))
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

	default:
		key, err := crypto.NewSSHKey(internal.GetString("SECURITY_SSH_KEY_TYPE", "KEY_TYPE_ED25519"))
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedCreatingSSHKey)
		}

		keyAccess := map[string]string{"public": "" + string(key.PublicKey) + "", "private": "" + string(key.PrivateKey) + "", "password": ""}
		keyAccessJSON, _ := json.Marshal(keyAccess)
		sqlAccess, err = sanitize.SQL(`
      INSERT INTO "server_access" ("server_id", "auth", "login", "key")
      VALUES ($1, '2', $2, $3)
      RETURNING "id"
    `,
			response.GetServerId(),
			in.GetLogin(),
			in.GetPassword(),
			string(keyAccessJSON),
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}
	}

	err = tx.QueryRowContext(ctx, sqlAccess).Scan(&accessID)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
	}

	_, err = tx.ExecContext(ctx, `
    UPDATE "server"
    SET "access_id" = $1
    WHERE "id" = $2
  `, accessID, response.GetServerId())
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	// + record server_access_policy
	_, err = tx.ExecContext(ctx, `
    INSERT INTO "server_access_policy" ("server_id", "ip", "country")
    VALUES ($1, 'f', 'f')
  `, response.GetServerId())
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
	}

	// + record server_activity
	sqlCountDay := `
    INSERT INTO "server_activity" ("server_id", "dow", "time_from", "time_to")
    VALUES
  `
	for countDay := 1; countDay < 8; countDay++ {
		for countHour := 0; countHour < 24; countHour++ {
			timeFrom := fmt.Sprintf("%02v:00:00", strconv.Itoa(countHour))
			timeTo := fmt.Sprintf("%02v:59:59", strconv.Itoa(countHour))
			sqlCountDay += fmt.Sprintf(`('%v', %v, '%v', '%v'),`, response.GetServerId(), countDay, timeFrom, timeTo)
		}
	}
	sqlCountDay = strings.TrimSuffix(sqlCountDay, ",")

	_, err = tx.ExecContext(ctx, sqlCountDay)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
	}

	if err := tx.Commit(); err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCommitError)
	}

	return response, nil
}

// UpdateServer is ...
func (h *Handler) UpdateServer(ctx context.Context, in *serverpb.UpdateServer_Request) (*serverpb.UpdateServer_Response, error) {
	switch in.GetSetting().(type) {
	case *serverpb.UpdateServer_Request_Info:
		tx, err := h.DB.Conn.BeginTx(ctx, nil)
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgTransactionCreateError)
		}
		defer tx.Rollback()

		_, err = tx.ExecContext(ctx, `
      UPDATE "server"
      SET
        "address" = $1,
        "port" = $2,
        "title" = $3,
        "description" = $4
      FROM "project"
      WHERE
        "server"."project_id" = "project"."id"
        AND "server"."id" = $5
        AND "project"."id" = $6
        AND "project"."owner_id" = $7
    `,
			in.GetInfo().GetAddress(),
			in.GetInfo().GetPort(),
			in.GetInfo().GetTitle(),
			in.GetInfo().GetDescription(),
			in.GetServerId(),
			in.GetProjectId(),
			in.GetUserId(),
		)
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
		}

		_, err = tx.ExecContext(ctx, `
      UPDATE "server_access"
      SET "login" = $1
      FROM
        "server"
        INNER JOIN "project" ON "server"."project_id" = "project"."id"
      WHERE
        "server"."access_id" = "server_access"."id"
        AND "server"."id" = $2
        AND "project"."id" = $3
        AND "project"."owner_id" = $4
    `,
			in.GetInfo().GetLogin(),
			in.GetServerId(),
			in.GetProjectId(),
			in.GetUserId(),
		)
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
		}

		if err := tx.Commit(); err != nil {
			return nil, trace.Error(err, log, trace.MsgTransactionCommitError)
		}

	case *serverpb.UpdateServer_Request_Audit:
		_, err := h.DB.Conn.ExecContext(ctx, `
      UPDATE "server"
      SET "audit" = $1
      FROM "project"
      WHERE
        "server"."project_id" = "project"."id"
        AND "server"."id" = $2
        AND "project"."id" = $3
        AND "project"."owner_id" = $4
    `,
			in.GetAudit(),
			in.GetServerId(),
			in.GetProjectId(),
			in.GetUserId(),
		)
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
		}

	case *serverpb.UpdateServer_Request_Active:
		// TODO After turning off, turn off all users who online
		_, err := h.DB.Conn.ExecContext(ctx, `
      UPDATE "server"
      SET "active" = $1
      FROM "project"
      WHERE
        "server"."project_id" = "project"."id"
        AND "server"."id" = $2
        AND "project"."id" = $3
        AND "project"."owner_id" = $4
    `,
			in.GetActive(),
			in.GetServerId(),
			in.GetProjectId(),
			in.GetUserId(),
		)
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
		}

	case *serverpb.UpdateServer_Request_Online:
		_, err := h.DB.Conn.ExecContext(ctx, `
      UPDATE "server"
      SET "online" = $1
      FROM "project"
      WHERE
        "server"."project_id" = "project"."id"
        AND "server"."id" = $2
        AND "project"."id" = $3
        AND "project"."owner_id" = $4
    `,
			in.GetOnline(),
			in.GetServerId(),
			in.GetProjectId(),
			in.GetUserId(),
		)
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
		}
	default:
		errGRPC := status.Error(codes.Aborted, trace.MsgInvalidArgument)
		return nil, trace.Error(errGRPC, log, nil)
	}
	return &serverpb.UpdateServer_Response{}, nil
}

// DeleteServer is ...
func (h *Handler) DeleteServer(ctx context.Context, in *serverpb.DeleteServer_Request) (*serverpb.DeleteServer_Response, error) {
	_, err := h.DB.Conn.ExecContext(ctx, `
    DELETE FROM "server"
    USING "project"
    WHERE
      "server"."project_id" = "project"."id"
      AND "server"."id" = $1
      AND "project"."id" = $2
      AND "project"."owner_id" = $3
  `, in.GetServerId(), in.GetProjectId(), in.GetUserId())
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToDelete)
	}
	return &serverpb.DeleteServer_Response{}, nil
}

// ServerAccess is displays an affordable version of connecting to the server
func (h *Handler) ServerAccess(ctx context.Context, in *serverpb.ServerAccess_Request) (*serverpb.ServerAccess_Response, error) {
	var password, publicKey, privateKey, privateKeyPassword sql.NullString
	response := &serverpb.ServerAccess_Response{}
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "server_access"."auth",
      "server_access"."login",
      "server_access"."password",
      "server_access"."key" ->> 'public',
      "server_access"."key" ->> 'private',
      "server_access"."key" ->> 'password'
    FROM
      "server"
      INNER JOIN "server_access" ON "server"."access_id" = "server_access"."id"
      INNER JOIN "project" ON "server"."project_id" = "project"."id"
    WHERE
      "server"."id" = $1
      AND "project"."id" = $2
      AND "project"."owner_id" = $3
  `, in.GetServerId(), in.GetProjectId(), in.GetUserId(),
	).Scan(&response.Auth,
		&response.Login,
		&password,
		&publicKey,
		&privateKey,
		&privateKeyPassword,
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	switch response.GetAuth() {
	case serverpb.Auth_password:
		response.Access = &serverpb.ServerAccess_Response_Password{
			Password: password.String,
		}
	case serverpb.Auth_key:
		response.Auth = serverpb.Auth_key
		response.Access = &serverpb.ServerAccess_Response_Key{
			Key: &serverpb.ServerAccess_Key{
				Public: publicKey.String,
			},
		}
	}

	return response, nil
}

// UpdateServerAccess is updates an affordable option for connecting to the server
func (h *Handler) UpdateServerAccess(ctx context.Context, in *serverpb.UpdateServerAccess_Request) (*serverpb.UpdateServerAccess_Response, error) {
	var sqlQuery string
	var err error
	response := &serverpb.UpdateServerAccess_Response{}

	switch in.GetAccess().(type) {
	case *serverpb.UpdateServerAccess_Request_Password:
		sqlQuery, err = sanitize.SQL(`
      UPDATE "server_access"
      SET "password" = $1
      FROM
        "server"
        INNER JOIN "project" ON "server"."project_id" = "project"."id"
      WHERE
        "server"."access_id" = "server_access"."id"
        AND "server"."id" = $2
        AND "project"."id" = $3
        AND "project"."owner_id" = $4
    `,
			in.GetPassword(),
			in.GetServerId(),
			in.GetProjectId(),
			in.GetUserId(),
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

	case *serverpb.UpdateServerAccess_Request_Key:
		cacheKey, err := h.Redis.Client.Get(ctx, fmt.Sprintf("tmp_key_ssh:%s", in.GetKey())).Result()
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		h.Redis.Client.Del(ctx, fmt.Sprintf("tmp_key_ssh:%s", in.GetKey()))
		sqlQuery, err = sanitize.SQL(`
      UPDATE "server_access"
      SET "key" = $1
      FROM
        "server"
        INNER JOIN "project" ON "server"."project_id" = "project"."id"
      WHERE
        "server"."access_id" = "server_access"."id"
        AND "server"."id" = $2
        AND "project"."id" = $3
        AND "project"."owner_id" = $4
    `,
			cacheKey,
			in.GetServerId(),
			in.GetProjectId(),
			in.GetUserId(),
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

	default:
		return response, nil
	}

	_, err = h.DB.Conn.ExecContext(ctx, sqlQuery)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	return response, nil
}

// ServerActivity is ...
func (h *Handler) ServerActivity(ctx context.Context, in *serverpb.ServerActivity_Request) (*serverpb.ServerActivity_Response, error) {
	response := &serverpb.ServerActivity_Response{}
	response.Monday = make([]int32, 24)
	response.Tuesday = make([]int32, 24)
	response.Wednesday = make([]int32, 24)
	response.Thursday = make([]int32, 24)
	response.Friday = make([]int32, 24)
	response.Saturday = make([]int32, 24)
	response.Sunday = make([]int32, 24)

	type dayActive struct {
		activityID string
		week       int32
		hour       int32
	}

	days := []dayActive{}
	rows, err := h.DB.Conn.QueryContext(ctx, `
    SELECT
      "server_activity"."id"  AS "activity_id",
      "server_activity"."dow" AS "week",
      EXTRACT(
        HOUR
        FROM
          "server_activity"."time_from"
      ) AS "hour"
    FROM
      "server"
      INNER JOIN "server_activity" ON "server"."id" = "server_activity"."server_id"
      INNER JOIN "project" ON "server"."project_id" = "project"."id"
    WHERE
      "server"."id" = $1
      AND "project"."id" = $2
      AND "project"."owner_id" = $3
  `, in.GetServerId(), in.GetProjectId(), in.GetUserId())
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	for rows.Next() {
		day := dayActive{}
		if err := rows.Scan(&day.activityID, &day.week, &day.hour); err != nil {
			return nil, trace.Error(err, log, nil)
		}
		days = append(days, day)
	}
	defer rows.Close()

	for _, item := range days {
		var status int32
		if item.activityID != "" {
			status = 1
		}

		switch item.week {
		case 1:
			response.Monday[item.hour] = status
		case 2:
			response.Tuesday[item.hour] = status
		case 3:
			response.Wednesday[item.hour] = status
		case 4:
			response.Thursday[item.hour] = status
		case 5:
			response.Friday[item.hour] = status
		case 6:
			response.Saturday[item.hour] = status
		case 7:
			response.Sunday[item.hour] = status
		}
	}

	return response, nil
}

// UpdateServerActivity is ...
func (h *Handler) UpdateServerActivity(ctx context.Context, in *serverpb.UpdateServerActivity_Request) (*serverpb.UpdateServerActivity_Response, error) {
	response := &serverpb.UpdateServerActivity_Response{}
	sqlQuery := map[string]string{
		"add": "",
		"del": "",
	}
	week := map[int32]string{
		1: "Monday",
		2: "Tuesday",
		3: "Wednesday",
		4: "Thursday",
		5: "Friday",
		6: "Saturday",
		7: "Sunday",
	}

	oldActivity, err := h.ServerActivity(ctx, &serverpb.ServerActivity_Request{
		UserId:    in.GetUserId(),
		ServerId:  in.GetServerId(),
		ProjectId: in.GetProjectId(),
	})
	if err != nil {
		errorInfo := trace.ParseError(err)
		if errorInfo.Code == codes.NotFound {
			errGRPC := status.Error(codes.NotFound, errorInfo.Message)
			return nil, trace.Error(errGRPC, log, nil)
		}
		return nil, trace.Error(err, log, nil)
	}

	_oldActivity := reflect.ValueOf(oldActivity)
	_newActivity := reflect.ValueOf(in.GetActivity())
	for index := range week {
		oldDay := reflect.Indirect(_oldActivity).FieldByName(week[index])
		newDay := reflect.Indirect(_newActivity).FieldByName(week[index])

		for hour := range oldDay.Interface().([]int32) {
			oldDayTmp := oldDay.Interface().([]int32)[hour]
			newDayTmp := newDay.Interface().([]int32)[hour]

			if oldDayTmp != newDayTmp {
				if oldDayTmp > newDayTmp {
					sqlQuery["del"] += fmt.Sprintf(` ("server_id" = '%s' AND "dow" = %v AND "time_from" = '%v:00:00') OR`,
						in.GetServerId(),
						index,
						hour,
					)
				} else {
					sqlQuery["add"] += fmt.Sprintf(` ('%s', %v, '%v:00:00', '%v:59:59'),`,
						in.GetServerId(),
						index,
						hour,
						hour,
					)
				}
			}
		}
	}

	if sqlQuery["del"] != "" {
		sqlQuery["del"] = fmt.Sprintf(`DELETE FROM "server_activity" WHERE %s`,
			sqlQuery["del"][:len(sqlQuery["del"])-2],
		)
		_, err := h.DB.Conn.ExecContext(ctx, sqlQuery["del"])
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToDelete)
		}
	}

	if sqlQuery["add"] != "" {
		sqlQuery["add"] = fmt.Sprintf(`
    INSERT INTO "server_activity" ("server_id", "dow", "time_from", "time_to") VALUES %s
    `, sqlQuery["add"][:len(sqlQuery["add"])-1])
		_, err := h.DB.Conn.ExecContext(ctx, sqlQuery["add"])
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToAdd)
		}
	}

	return response, nil
}

// ListShareServers is ...
func (h *Handler) ListShareServers(ctx context.Context, in *serverpb.ListShareServers_Request) (*serverpb.ListShareServers_Response, error) {
	response := &serverpb.ListShareServers_Response{}
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := h.DB.Conn.QueryContext(ctx, `
    SELECT
      "user"."login"                AS user_login,
      "project"."login"             AS project_login,
      "project"."title"             AS project_title,
      "server"."token"              AS server_token,
      "server"."id"                 AS server_id,
      "server"."online"             AS server_online,
      "server"."title"              AS server_title,
      "server"."public_description" AS server_description
    FROM
      "server"
      INNER JOIN "project" ON "server"."project_id" = "project"."id"
      INNER JOIN "project_member" ON "project"."id" = "project_member"."project_id"
      INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
    WHERE "project_member"."user_id" = $1
    `+sqlFooter, in.GetUserId())
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	for rows.Next() {
		var projectLogin, projectTitle string
		server := &serverpb.ListShareServers_Response_SharedServer{}
		err = rows.Scan(&server.UserLogin,
			&projectLogin,
			&projectTitle,
			&server.ServerToken,
			&server.ServerId,
			&server.ServerOnline,
			&server.ServerTitle,
			&server.ServerDescription,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		server.ProjectLogin = projectLogin
		response.Servers = append(response.Servers, server)
	}
	defer rows.Close()

	// Total count for pagination
	err = h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM
      "server"
      INNER JOIN "project_member" ON "server"."project_id" = "project_member"."project_id"
    WHERE "project_member"."user_id" = $1
  `, in.GetUserId(),
	).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		return nil, trace.Error(err, log, nil)
	}

	return response, nil
}

// AddShareServer is ...
func (h *Handler) AddShareServer(ctx context.Context, in *serverpb.AddShareServer_Request) (*serverpb.AddShareServer_Response, error) {
	return &serverpb.AddShareServer_Response{}, nil
}

// UpdateShareServer is ...
func (h *Handler) UpdateShareServer(ctx context.Context, in *serverpb.UpdateShareServer_Request) (*serverpb.UpdateShareServer_Response, error) {
	return &serverpb.UpdateShareServer_Response{}, nil
}

// DeleteShareServer is ...
func (h *Handler) DeleteShareServer(ctx context.Context, in *serverpb.DeleteShareServer_Request) (*serverpb.DeleteShareServer_Response, error) {
	response := &serverpb.DeleteShareServer_Response{}
	return response, nil
}

// UpdateHostKey is ...
func (h *Handler) UpdateHostKey(ctx context.Context, in *serverpb.UpdateHostKey_Request) (*serverpb.UpdateHostKey_Response, error) {
	_, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "server_host_key"
    SET "host_key" = $1
    WHERE "server_id" = $2
  `, in.GetHostkey(), in.GetServerId())
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	return &serverpb.UpdateHostKey_Response{}, nil
}

// AddSession is ...
func (h *Handler) AddSession(ctx context.Context, in *serverpb.AddSession_Request) (*serverpb.AddSession_Response, error) {
	response := &serverpb.AddSession_Response{}
	err := h.DB.Conn.QueryRowContext(ctx, `
    INSERT INTO "session" ("account_id", "status", "message")
    VALUES ($1, $2, $3)
    RETURNING "id"
  `, in.GetAccountId(), strings.ToLower(in.Status.String()), in.GetMessage(),
	).Scan(&response.SessionId)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
	}

	return response, nil
}

// ServerNameByID is ...
func (h *Handler) ServerNameByID(ctx context.Context, in *serverpb.ServerNameByID_Request) (*serverpb.ServerNameByID_Response, error) {
	response := &serverpb.ServerNameByID_Response{}
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT "server"."title"
    FROM
      "server"
      INNER JOIN "server_access" ON "server"."access_id" = "server_access"."id"
      INNER JOIN "project" ON "server"."project_id" = "project"."id"
    WHERE
      "server"."id" = $1
      AND "project"."id" = $2
      AND "project"."owner_id" = $3
  `, in.GetServerId(), in.GetProjectId(), in.GetUserId(),
	).Scan(&response.ServerName)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	return response, nil
}
