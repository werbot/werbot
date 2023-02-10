package server

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/werbot/werbot/internal/crypto"
	"github.com/werbot/werbot/internal/grpc/project"
	serverpb "github.com/werbot/werbot/internal/grpc/server/proto"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/pkg/strutil"
)

// TODO: a method for updating Host FingerPrint server
// TODO: When updating the IP address of the server, you need to update HostKey !!!!

// ListServers is displays a list of available servers
func (h *Handler) ListServers(ctx context.Context, in *serverpb.ListServers_Request) (*serverpb.ListServers_Response, error) {
	// TODO: checking the permitted servers available for display
	//  log.Info().Msgf("Expired: %v", license.L.GetCustomer())

	response := new(serverpb.ListServers_Response)

	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	query := h.DB.QueryParse(in.GetQuery())

	if query["login"] != "" {
		loginArray := strutil.SplitNTrimmed(query["login"], "_", 3)

		nameLen := len(loginArray)
		query := `SELECT DISTINCT ON ("server"."id")
        "server"."id",
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
      FROM "user"
        JOIN "project_member" ON "user"."id" = "project_member"."user_id"
        JOIN "project" ON "project"."id" = "project_member"."project_id"
        JOIN "server" ON "project"."id" = "server"."project_id"
        JOIN "server_host_key" ON "server_host_key"."server_id" = "server"."id"
        JOIN "server_member" ON "server_member"."server_id" = "server"."id"
        AND "server_member"."member_id" = "project_member"."id"
				INNER JOIN "server_access" ON "server"."access_id" = "server_access"."id"
      WHERE "user"."login" = $1
        AND "server_member"."active" = TRUE
        AND "server"."active" = TRUE`

		switch nameLen {
		case 1:
			rows, err := h.DB.Conn.Query(query, loginArray[0])
			if err != nil {
				log.FromGRPC(err).Send()
				return nil, errServerError
			}

			for rows.Next() {
				server := new(serverpb.Server_Response)
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
					if err == sql.ErrNoRows {
						return nil, errNotFound
					}
					log.FromGRPC(err).Send()
					return nil, errServerError
				}
				response.Servers = append(response.Servers, server)
			}
			defer rows.Close()

		case 2:
			rows, err := h.DB.Conn.Query(query+`
				AND "project"."login" = $2`, loginArray[0], loginArray[1])
			if err != nil {
				log.FromGRPC(err).Send()
				return nil, errServerError
			}

			for rows.Next() {
				server := new(serverpb.Server_Response)
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
					if err == sql.ErrNoRows {
						return nil, errNotFound
					}
					log.FromGRPC(err).Send()
					return nil, errServerError
				}
				response.Servers = append(response.Servers, server)
			}
			defer rows.Close()

		case 3:
			rows, err := h.DB.Conn.Query(query+`
				AND "project"."login" = $2
				AND "token" = $3
				AND "project_member"."role" = 'user'`, loginArray[0], loginArray[1], loginArray[2])
			if err != nil {
				log.FromGRPC(err).Send()
				return nil, errServerError
			}

			for rows.Next() {
				server := new(serverpb.Server_Response)
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
					if err == sql.ErrNoRows {
						return nil, errNotFound
					}
					log.FromGRPC(err).Send()
					return nil, errServerError
				}
				response.Servers = append(response.Servers, server)
			}
			defer rows.Close()
		}

		response.Total = int32(len(response.Servers))
		if response.Total == 0 {
			return nil, errNotFound
		}

		return response, nil
	}

	if query["project_id"] != "" && query["user_id"] != "" {
		rows, err := h.DB.Conn.Query(`SELECT DISTINCT ON ("server"."id")
				"server"."id",
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
				(SELECT COUNT (*) FROM "server_member" WHERE "server_id" = "server"."id") AS "count_members"
			FROM "server"
				INNER JOIN "project" ON "server"."project_id" = "project"."id"
				INNER JOIN "server_access" ON "server"."access_id" = "server_access"."id"
			WHERE "server"."project_id" = $1
				AND "project"."owner_id" = $2`+sqlFooter,
			query["project_id"],
			query["user_id"],
		)
		if err != nil {
			log.FromGRPC(err).Send()
			return nil, errServerError
		}

		for rows.Next() {
			server := new(serverpb.Server_Response)
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
				if err == sql.ErrNoRows {
					return nil, errNotFound
				}
				log.FromGRPC(err).Send()
				return nil, errServerError
			}
			response.Servers = append(response.Servers, server)
		}
		defer rows.Close()

		// Total count
		err = h.DB.Conn.QueryRow(`SELECT COUNT(*)
      FROM "server"
				INNER JOIN "project" ON "server"."project_id" = "project"."id"
			WHERE "server"."project_id" = $1
				AND "project"."owner_id" = $2`,
			query["project_id"],
			query["user_id"],
		).Scan(&response.Total)
		if err != nil && err != sql.ErrNoRows {
			log.FromGRPC(err).Send()
			return nil, errServerError
		}

		return response, nil
	}

	return response, nil
}

// Server is displays data on the server
func (h *Handler) Server(ctx context.Context, in *serverpb.Server_Request) (*serverpb.Server_Response, error) {
	if !project.IsOwnerProject(h.DB, in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var description pgtype.Text
	response := new(serverpb.Server_Response)

	err := h.DB.Conn.QueryRow(`SELECT
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
		FROM "server"
			INNER JOIN "server_access" ON "server"."access_id" = "server_access"."id"
		WHERE "server"."id" = $1
			AND "server"."project_id" = $2`,
		in.GetServerId(),
		in.GetProjectId(),
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
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	response.Description = description.String

	return response, nil
}

// AddServer is ...
func (h *Handler) AddServer(ctx context.Context, in *serverpb.AddServer_Request) (*serverpb.AddServer_Response, error) {
	if !project.IsOwnerProject(h.DB, in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var err error
	response := new(serverpb.AddServer_Response)

	/*
		var serverPassword string
		var serverKeys = new(crypto.PairOfKeys)
		var auth serverpb.Auth


		switch in.GetAccess().(type) {
		case *serverpb.AddServer_Request_Password:
			auth = serverpb.Auth_password
			serverPassword = in.GetPassword()

		case *serverpb.AddServer_Request_KeyUuid:
			auth = serverpb.Auth_key
			if in.GetKeyUuid() != "" {
				key := new(keypb.GenerateSSHKey_Key)
				if err := service.cache.Get(fmt.Sprintf("tmp_key_ssh::%s", in.GetKeyUuid())).Scan(key); err != nil {
					log.FromGRPC(err).Send()
					return nil, errServerError
				}

				service.cache.Delete(fmt.Sprintf("tmp_key_ssh::%s", in.GetKeyUuid()))
				serverKeys.PrivateKey = []byte(key.GetPrivate())
				serverKeys.PublicKey = []byte(key.GetPublic())
			} else {
				serverKeys, err = crypto.NewSSHKey(internal.GetString("SECURITY_SSH_KEY_TYPE", "KEY_TYPE_ED25519"))
				if err != nil {
					log.FromGRPC(err).Send()
					return nil, crypto.ErrFailedCreatingSSHKey
				}
			}
			response.KeyPublic = string(serverKeys.PublicKey)
		}
	*/

	serverToken := crypto.NewPassword(6, false)
	serverTitle := in.GetTitle()
	if in.Title == "" {
		serverTitle = fmt.Sprintf("server #%s", serverToken)
	}

	tx, err := h.DB.Conn.Begin()
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errTransactionCreateError
	}

	// TODO: This design converts the number into a line into an old format that is registered in the database,
	// I recommend that you store numerical values in the new format in the database
	//serverScheme := strings.ToLower(serverpb.ServerScheme_name[int32(in.Scheme.Number())])

	err = tx.QueryRow(`INSERT INTO "server" (
			"project_id",
			"address",
			"port",
			"token",
			"description",
			"title",
			"active",
			"audit",
			"scheme",
			"previous_state"
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, '[]')
		RETURNING "id"`,
		in.GetProjectId(),
		in.GetAddress(),
		in.GetPort(),
		serverToken,
		in.GetDescription(),
		serverTitle,
		in.GetActive(),
		in.GetAudit(),
		in.GetScheme().Enum(),
	).Scan(&response.ServerId)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	// + record server_access_policy
	data, err := tx.Exec(`INSERT INTO "server_access_policy" ("server_id", "ip", "country") VALUES ($1, 'f', 'f')`,
		response.GetServerId(),
	)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	// + record server_activity
	sqlCountDay := `INSERT INTO "server_activity" ("server_id", "dow", "time_from", "time_to") VALUES `
	for countDay := 1; countDay < 8; countDay++ {
		for countHour := 0; countHour < 24; countHour++ {
			timeFrom := fmt.Sprintf("%02v:00:00", strconv.Itoa(countHour))
			timeTo := fmt.Sprintf("%02v:59:59", strconv.Itoa(countHour))
			sqlCountDay += fmt.Sprintf(`(%v, %v, '%v', '%v'),`, response.GetServerId(), countDay, timeFrom, timeTo)
		}
	}
	sqlCountDay = strings.TrimSuffix(sqlCountDay, ",")

	data, err = tx.Exec(sqlCountDay)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	if err = tx.Commit(); err != nil {
		log.FromGRPC(err).Send()
		return nil, errTransactionCommitError
	}

	return response, nil
}

// UpdateServer is ...
func (h *Handler) UpdateServer(ctx context.Context, in *serverpb.UpdateServer_Request) (*serverpb.UpdateServer_Response, error) {
	if !project.IsOwnerProject(h.DB, in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var err error
	var data sql.Result
	response := new(serverpb.UpdateServer_Response)

	switch in.GetSetting().(type) {
	case *serverpb.UpdateServer_Request_Info:
		data, err = h.DB.Conn.Exec(`UPDATE "server"
    SET
      "address" = $1,
      "port" = $2,
      "title" = $3,
      "description" = $4,
      "last_update" = NOW()
    WHERE "id" = $5 AND "project_id" = $6`,
			in.GetInfo().GetAddress(),
			in.GetInfo().GetPort(),
			in.GetInfo().GetTitle(),
			in.GetInfo().GetDescription(),
			in.GetServerId(),
			in.GetProjectId(),
		)

	case *serverpb.UpdateServer_Request_Audit:
		data, err = h.DB.Conn.Exec(`UPDATE "server" SET "audit" = $1, "last_update" = NOW() WHERE "id" = $2`,
			in.GetAudit(),
			in.GetServerId(),
		)

	case *serverpb.UpdateServer_Request_Active:
		// TODO After turning off, turn off all users who online
		data, err = h.DB.Conn.Exec(`UPDATE "server" SET "active" = $1, "last_update" = NOW() WHERE "id" = $2`,
			in.GetActive(),
			in.GetServerId(),
		)

	case *serverpb.UpdateServer_Request_Online:
		data, err = h.DB.Conn.Exec(`UPDATE "server" SET "online" = $1, "last_update" = NOW() WHERE "id" = $2`,
			in.GetOnline(),
			in.GetServerId(),
		)

	default:
		return nil, errBadRequest
	}

	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// DeleteServer is ...
func (h *Handler) DeleteServer(ctx context.Context, in *serverpb.DeleteServer_Request) (*serverpb.DeleteServer_Response, error) {
	if !project.IsOwnerProject(h.DB, in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	response := new(serverpb.DeleteServer_Response)

	data, err := h.DB.Conn.Exec(`DELETE FROM "server" WHERE "id" = $1 AND "project_id" = $2`,
		in.GetServerId(),
		in.GetProjectId(),
	)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// ServerAccess is displays an affordable version of connecting to the server
func (h *Handler) ServerAccess(ctx context.Context, in *serverpb.ServerAccess_Request) (*serverpb.ServerAccess_Response, error) {
	if !project.IsOwnerProject(h.DB, in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var password, publicKey, privateKey, privateKeyPassword sql.NullString
	response := new(serverpb.ServerAccess_Response)

	err := h.DB.Conn.QueryRow(`SELECT "auth",
			"login",
			"password",
      "key"->>'public',
      "key"->>'private',
      "key"->>'password'
		FROM "server_access"
		WHERE "server_id" = $1`,
		in.GetServerId(),
	).Scan(&response.Auth,
		&response.Login,
		&password,
		&publicKey,
		&privateKey,
		&privateKeyPassword,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		log.FromGRPC(err).Send()
		return nil, errServerError
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

// AddServerAccess is ..
func (h *Handler) AddServerAccess(ctx context.Context, in *serverpb.AddServerAccess_Request) (*serverpb.AddServerAccess_Response, error) {
	response := new(serverpb.AddServerAccess_Response)

	return response, nil
}

// UpdateServerAccess is updates an affordable option for connecting to the server
func (h *Handler) UpdateServerAccess(ctx context.Context, in *serverpb.UpdateServerAccess_Request) (*serverpb.UpdateServerAccess_Response, error) {
	if !project.IsOwnerProject(h.DB, in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var sqlQuery string
	response := new(serverpb.UpdateServerAccess_Response)

	switch in.GetAccess().(type) {
	case *serverpb.UpdateServerAccess_Request_Password:
		sqlQuery, _ = sanitize.SQL(`UPDATE "server_access" SET "password" = $1, "last_update" = NOW() WHERE "server_id" = $2`,
			in.GetPassword(),
			in.GetServerId(),
		)
	case *serverpb.UpdateServerAccess_Request_Key:
		cacheKey, err := h.Redis.Get(fmt.Sprintf("tmp_key_ssh::%s", in.GetKey())).Result()
		if err != nil {
			log.FromGRPC(err).Send()
			return nil, errServerError
		}

		h.Redis.Delete(fmt.Sprintf("tmp_key_ssh::%s", in.GetKey()))
		sqlQuery, _ = sanitize.SQL(`UPDATE "server_access" SET "key" = $1, "last_update" = NOW() WHERE "server_id" = $2 `,
			cacheKey,
			in.GetServerId(),
		)
	default:
		return response, nil
	}

	data, err := h.DB.Conn.Exec(sqlQuery)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// ServerActivity is ...
func (h *Handler) ServerActivity(ctx context.Context, in *serverpb.ServerActivity_Request) (*serverpb.ServerActivity_Response, error) {
	if !project.IsOwnerProject(h.DB, in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	response := new(serverpb.ServerActivity_Response)
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

	rows, err := h.DB.Conn.Query(`SELECT
			"server_activity"."id" AS "activity_id",
			"server_activity"."dow" AS "week",
			EXTRACT (HOUR FROM "server_activity"."time_from") AS "hour"
		FROM "server_activity"
			INNER JOIN "server" ON "server_activity"."server_id" = "server"."id"
		WHERE "server_activity"."server_id" = $1
			AND "server"."project_id" = $2`,
		in.GetServerId(),
		in.GetProjectId(),
	)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	for rows.Next() {
		day := dayActive{}
		if err := rows.Scan(&day.activityID, &day.week, &day.hour); err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			log.FromGRPC(err).Send()
			return nil, errServerError
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
	response := new(serverpb.UpdateServerActivity_Response)
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
		log.FromGRPC(err).Send()
		return nil, err
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
		data, err := h.DB.Conn.Exec(sqlQuery["del"])
		if err != nil {
			log.FromGRPC(err).Send()
			return nil, errFailedToDelete
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}
	}

	if sqlQuery["add"] != "" {
		sqlQuery["add"] = fmt.Sprintf(`INSERT INTO "server_activity" ("server_id", "dow", "time_from", "time_to") VALUES %s`,
			sqlQuery["add"][:len(sqlQuery["add"])-1],
		)
		data, err := h.DB.Conn.Exec(sqlQuery["add"])
		if err != nil {
			log.FromGRPC(err).Send()
			return nil, errFailedToAdd
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}
	}

	return response, nil
}

// ListShareServers is ...
func (h *Handler) ListShareServers(ctx context.Context, in *serverpb.ListShareServers_Request) (*serverpb.ListShareServers_Response, error) {
	response := new(serverpb.ListShareServers_Response)

	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := h.DB.Conn.Query(`SELECT
			"user"."login" AS user_login,
			"project"."login" AS project_login,
			"project"."title" AS project_title,
			"server"."token" AS server_token,
			"server"."id" AS server_id,
			"server"."online" AS server_online,
			"server"."title" AS server_title,
			"server"."public_description" AS server_description
		FROM "server"
			INNER JOIN "project" ON "server"."project_id" = "project"."id"
			INNER JOIN "project_member" ON "project"."id" = "project_member"."project_id"
			INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
		WHERE "project_member"."user_id" = $1`+sqlFooter,
		in.GetUserId(),
	)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	for rows.Next() {
		var projectLogin, projectTitle string
		server := new(serverpb.ListShareServers_Response_SharedServer)
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
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			log.FromGRPC(err).Send()
			return nil, errServerError
		}
		server.ProjectLogin = projectLogin

		response.Servers = append(response.Servers, server)
	}
	defer rows.Close()

	// Total count for pagination
	err = h.DB.Conn.QueryRow(`SELECT COUNT(*)
		FROM "server"
			INNER JOIN "project_member" ON "server"."project_id" = "project_member"."project_id"
		WHERE "project_member"."user_id" = $1`,
		in.GetUserId(),
	).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	return response, nil
}

// AddShareServer is ...
func (h *Handler) AddShareServer(ctx context.Context, in *serverpb.AddShareServer_Request) (*serverpb.AddShareServer_Response, error) {
	response := new(serverpb.AddShareServer_Response)
	return response, nil
}

// UpdateShareServer is ...
func (h *Handler) UpdateShareServer(ctx context.Context, in *serverpb.UpdateShareServer_Request) (*serverpb.UpdateShareServer_Response, error) {
	response := new(serverpb.UpdateShareServer_Response)
	return response, nil
}

// DeleteShareServer is ...
func (h *Handler) DeleteShareServer(ctx context.Context, in *serverpb.DeleteShareServer_Request) (*serverpb.DeleteShareServer_Response, error) {
	response := new(serverpb.DeleteShareServer_Response)
	return response, nil
}

// UpdateHostKey is ...
func (h *Handler) UpdateHostKey(ctx context.Context, in *serverpb.UpdateHostKey_Request) (*serverpb.UpdateHostKey_Response, error) {
	response := new(serverpb.UpdateHostKey_Response)

	data, err := h.DB.Conn.Exec(`UPDATE "server_host_key" SET "host_key" = $1, "last_update" = NOW() WHERE "server_id" = $2`,
		in.GetHostkey(),
		in.GetServerId(),
	)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// AddSession is ...
func (h *Handler) AddSession(ctx context.Context, in *serverpb.AddSession_Request) (*serverpb.AddSession_Response, error) {
	response := new(serverpb.AddSession_Response)

	err := h.DB.Conn.QueryRow(`INSERT INTO "session" ("account_id", "status", "message")
		VALUES ($1, $2, $3)
		RETURNING id`,
		in.GetAccountId(),
		strings.ToLower(in.Status.String()),
		in.GetMessage(),
	).Scan(&response.SessionId)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	return response, nil
}

// ServerNameByID is ...
func (h *Handler) ServerNameByID(ctx context.Context, in *serverpb.ServerNameByID_Request) (*serverpb.ServerNameByID_Response, error) {
	if !project.IsOwnerProject(h.DB, in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	response := new(serverpb.ServerNameByID_Response)

	err := h.DB.Conn.QueryRow(`SELECT "title" FROM "server" WHERE "id" = $1`,
		in.GetServerId(),
	).Scan(&response.ServerName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	return response, nil
}
