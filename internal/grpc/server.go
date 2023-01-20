package grpc

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgtype"

	serverpb "github.com/werbot/werbot/api/proto/server"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/crypto"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/pkg/strutil"
)

type server struct {
	serverpb.UnimplementedServerHandlersServer
}

// TODO: a method for updating Host FingerPrint server
// TODO: When updating the IP address of the server, you need to update HostKey !!!!

// GetServers is displays a list of available servers
func (s *server) ListServers(ctx context.Context, in *serverpb.ListServers_Request) (*serverpb.ListServers_Response, error) {
	// TODO: checking the permitted servers available for display
	//  log.Info().Msgf("Expired: %v", license.L.GetCustomer())

	response := new(serverpb.ListServers_Response)

	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	query := service.db.QueryParse(in.GetQuery())

	if query["login"] != "" {
		loginArray := strutil.SplitNTrimmed(query["login"], "_", 3)

		nameLen := len(loginArray)
		query := `SELECT DISTINCT ON ("server"."id")
        "server"."id",
        "server"."port",
        "server"."address",
        "server"."token",
        "server"."login",
        "server"."password",
        "server"."title",
        "server"."audit",
        "server"."online",
        "server"."public_key",
        "server"."private_key",
        "server"."private_key_password",
        "server"."auth",
        "server"."scheme",
        "server_host_key"."host_key",
        "server_member"."id" AS "account_id",
        "project_member"."project_id",
        "project"."login" AS "project_login"
      FROM "user"
        JOIN "project_member" ON "user"."id" = "project_member"."user_id"
        JOIN "project" ON "project"."id" = "project_member"."project_id"
        JOIN "server" ON "project"."id" = "server"."project_id"
        JOIN "server_host_key" ON "server_host_key"."server_id" = "server"."id"
        JOIN "server_member" ON "server_member"."server_id" = "server"."id"
        AND "server_member"."member_id" = "project_member"."id"
      WHERE "user"."login" = $1
        AND "server_member"."active" = TRUE
        AND "server"."active" = TRUE`

		switch nameLen {
		case 1:
			rows, err := service.db.Conn.Query(query, loginArray[0])
			if err != nil {
				service.log.FromGRPC(err).Send()
				return nil, errServerError
			}

			for rows.Next() {
				server := new(serverpb.Server_Response)
				err = rows.Scan(&server.ServerId,
					&server.Port,
					&server.Address,
					&server.Token,
					&server.Login,
					&server.Password,
					&server.Title,
					&server.Audit,
					&server.Online,
					&server.KeyPublic,
					&server.KeyPrivate,
					&server.KeyPassword,
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
					service.log.FromGRPC(err).Send()
					return nil, errServerError
				}
				response.Servers = append(response.Servers, server)
			}
			defer rows.Close()

		case 2:
			rows, err := service.db.Conn.Query(query+`
				AND "project"."login" = $2`, loginArray[0], loginArray[1])
			if err != nil {
				service.log.FromGRPC(err).Send()
				return nil, errServerError
			}

			for rows.Next() {
				server := new(serverpb.Server_Response)
				err = rows.Scan(&server.ServerId,
					&server.Port,
					&server.Address,
					&server.Token,
					&server.Login,
					&server.Password,
					&server.Title,
					&server.Audit,
					&server.Online,
					&server.KeyPublic,
					&server.KeyPrivate,
					&server.KeyPassword,
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
					service.log.FromGRPC(err).Send()
					return nil, errServerError
				}
				response.Servers = append(response.Servers, server)
			}
			defer rows.Close()

		case 3:
			rows, err := service.db.Conn.Query(query+`
				AND "project"."login" = $2
				AND "token" = $3
				AND "project_member"."role" = 'user'`, loginArray[0], loginArray[1], loginArray[2])
			if err != nil {
				service.log.FromGRPC(err).Send()
				return nil, errServerError
			}

			for rows.Next() {
				server := new(serverpb.Server_Response)
				err = rows.Scan(&server.ServerId,
					&server.Port,
					&server.Address,
					&server.Token,
					&server.Login,
					&server.Password,
					&server.Title,
					&server.Audit,
					&server.Online,
					&server.KeyPublic,
					&server.KeyPrivate,
					&server.KeyPassword,
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
					service.log.FromGRPC(err).Send()
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
		rows, err := service.db.Conn.Query(`SELECT
      DISTINCT ON ("server"."id")
				"server".id,
				"server".address,
				"server".port,
				"server".token,
				"server".login,
				"server".title,
				"server".audit,
				"server".online,
				"server".auth,
				"server".active,
				"server".scheme,
				"server".private_description,
				"server".public_description,
				(SELECT COUNT (*) FROM "server_member" WHERE "server_id" = "server"."id") AS "count_members"
			FROM "server"
				INNER JOIN "project" ON "server"."project_id" = "project"."id"
			WHERE "server"."project_id" = $1
				AND "project"."owner_id" = $2`+sqlFooter,
			query["project_id"],
			query["user_id"],
		)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errServerError
		}

		for rows.Next() {
			server := new(serverpb.Server_Response)
			err = rows.Scan(&server.ServerId,
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
				&server.PrivateDescription,
				&server.PublicDescription,
				&server.CountMembers,
			)
			if err != nil {
				if err == sql.ErrNoRows {
					return nil, errNotFound
				}
				service.log.FromGRPC(err).Send()
				return nil, errServerError
			}
			response.Servers = append(response.Servers, server)
		}
		defer rows.Close()

		// Total count
		err = service.db.Conn.QueryRow(`SELECT COUNT(*)
      FROM "server"
				INNER JOIN "project" ON "server"."project_id" = "project"."id"
			WHERE "server"."project_id" = $1
				AND "project"."owner_id" = $2`,
			query["project_id"],
			query["user_id"],
		).Scan(&response.Total)
		if err != nil && err != sql.ErrNoRows {
			service.log.FromGRPC(err).Send()
			return nil, errServerError
		}

		return response, nil
	}

	return response, nil
}

// Server is displays data on the server
func (s *server) Server(ctx context.Context, in *serverpb.Server_Request) (*serverpb.Server_Response, error) {
	if !isOwnerProject(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var privateDescription, publicDescription pgtype.Text
	response := new(serverpb.Server_Response)

	err := service.db.Conn.QueryRow(`SELECT
			"server"."address",
			"server"."port",
			"server"."token",
			"server"."login",
			"server"."private_description",
			"server"."public_description",
			"server"."title",
			"server"."active",
			"server"."audit",
			"server"."online",
			"server"."auth",
			"server"."scheme"
		FROM "server"
		WHERE "server"."id" = $1
			AND "server"."project_id" = $2`,
		in.GetServerId(),
		in.GetProjectId(),
	).Scan(&response.Address,
		&response.Port,
		&response.Token,
		&response.Login,
		&privateDescription,
		&publicDescription,
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
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	response.PrivateDescription = privateDescription.String
	response.PublicDescription = publicDescription.String

	return response, nil
}

// AddServer is ...
func (s *server) AddServer(ctx context.Context, in *serverpb.AddServer_Request) (*serverpb.AddServer_Response, error) {
	if !isOwnerProject(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var serverPassword string
	var serverKeys = new(crypto.PairOfKeys)
	var err error
	response := new(serverpb.AddServer_Response)

	switch in.GetAuth() {
	case serverpb.Auth_password:
		serverPassword = in.GetPassword()
	case serverpb.Auth_key:
		if in.GetPublicKey() != "" && in.GetKeyUuid() != "" {
			privateKey, err := service.cache.Get(fmt.Sprintf("tmp_key_ssh::%s", in.GetKeyUuid()))
			if err != nil {
				service.log.FromGRPC(err).Send()
				return nil, errServerError
			}
			service.cache.Delete(fmt.Sprintf("tmp_key_ssh::%s", in.GetKeyUuid()))
			serverKeys.PrivateKey = []byte(privateKey)
			serverKeys.PublicKey = []byte(in.PublicKey)
		} else {
			serverKeys, err = crypto.NewSSHKey(internal.GetString("SECURITY_SSH_KEY_TYPE", "KEY_TYPE_ED25519"))
			if err != nil {
				service.log.FromGRPC(err).Send()
				return nil, crypto.ErrFailedCreatingSSHKey
			}
		}
		response.KeyPublic = string(serverKeys.PublicKey)
	}

	serverToken := crypto.NewPassword(6, false)
	serverTitle := in.GetTitle()
	if in.Title == "" {
		serverTitle = fmt.Sprintf("server #%s", serverToken)
	}

	tx, err := service.db.Conn.Begin()
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errTransactionCreateError
	}

	// TODO: This design converts the number into a line into an old format that is registered in the database,
	// I recommend that you store numerical values in the new format in the database
	serverAuth := strings.ToLower(serverpb.Auth_name[int32(in.Auth.Number())])
	serverScheme := strings.ToLower(serverpb.ServerScheme_name[int32(in.Scheme.Number())])

	err = tx.QueryRow(`INSERT INTO "server" (
			"project_id",
			"address",
			"port",
			"token",
			"login",
			"password",
			"private_description",
			"public_description",
			"title",
			"active",
			"audit",
			"public_key",
			"private_key",
			"created",
			"auth",
			"scheme",
			"previous_state",
			"private_key_password")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, '[]', $18)
		RETURNING "id"`,
		in.GetProjectId(),
		in.GetAddress(),
		in.GetPort(),
		serverToken,
		in.GetLogin(),
		serverPassword,
		in.GetPrivateDescription(),
		in.GetPublicDescription(),
		serverTitle,
		in.GetActive(),
		in.GetAudit(),
		string(serverKeys.PublicKey),
		string(serverKeys.PrivateKey),
		time.Now(),
		serverAuth,
		serverScheme,
		serverKeys.Passphrase,
	).Scan(&response.ServerId)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	// + record server_access_policy
	data, err := tx.Exec(`INSERT INTO "server_access_policy" ("server_id", "ip", "country") VALUES ($1, 'f', 'f')`,
		response.GetServerId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
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
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	if err = tx.Commit(); err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errTransactionCommitError
	}

	return response, nil
}

// UpdateServer is ...
func (s *server) UpdateServer(ctx context.Context, in *serverpb.UpdateServer_Request) (*serverpb.UpdateServer_Response, error) {
	if !isOwnerProject(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var err error
	var data sql.Result
	response := new(serverpb.UpdateServer_Response)

	switch in.GetSetting().(type) {
	case *serverpb.UpdateServer_Request_Info:
		data, err = service.db.Conn.Exec(`UPDATE "server" SET
      "address" = $3,
      "port" = $4,
      "login" = $5,
      "title" = $6,
      "active" = $7,
      "audit" = $8,
      "private_description" = $9,
      "public_description" = $10
    WHERE "id" = $1
      AND "project_id" = $2`,
			in.GetServerId(),
			in.GetProjectId(),
			in.GetInfo().GetAddress(),
			in.GetInfo().GetPort(),
			in.GetInfo().GetLogin(),
			in.GetInfo().GetTitle(),
			in.GetActive(),
			in.GetAudit(),
			in.GetInfo().GetPrivateDescription(),
			in.GetInfo().GetPublicDescription(),
		)

	case *serverpb.UpdateServer_Request_Audit:
		data, err = service.db.Conn.Exec(`UPDATE "server" SET "audit" = $1 WHERE "id" = $2`,
			in.GetAudit(),
			in.GetServerId(),
		)

	case *serverpb.UpdateServer_Request_Active:
		// TODO After turning off, turn off all users who online
		data, err = service.db.Conn.Exec(`UPDATE "server" SET "active" = $1 WHERE "id" = $2`,
			in.GetActive(),
			in.GetServerId(),
		)

	case *serverpb.UpdateServer_Request_Online:
		data, err = service.db.Conn.Exec(`UPDATE "server" SET "online" = $1 WHERE "id" = $2`,
			in.GetOnline(),
			in.GetServerId(),
		)

	default:
		return nil, errBadRequest
	}

	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// DeleteServer is ...
func (s *server) DeleteServer(ctx context.Context, in *serverpb.DeleteServer_Request) (*serverpb.DeleteServer_Response, error) {
	if !isOwnerProject(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	response := new(serverpb.DeleteServer_Response)

	data, err := service.db.Conn.Exec(`DELETE FROM "server" WHERE "id" = $1 AND "project_id" = $2`,
		in.GetServerId(),
		in.GetProjectId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// ServerAccess is displays an affordable version of connecting to the server
func (s *server) ServerAccess(ctx context.Context, in *serverpb.ServerAccess_Request) (*serverpb.ServerAccess_Response, error) {
	if !isOwnerProject(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	response := new(serverpb.ServerAccess_Response)
	server := new(serverpb.Server_Response)

	err := service.db.Conn.QueryRow(`SELECT
			"server"."password",
			"server"."public_key",
			"server"."private_key",
			"server"."private_key_password",
			"server"."auth"
		FROM "server"
			INNER JOIN "project" ON "server"."project_id" = "project". "id"
		WHERE "server"."id" = $1
			AND "server"."project_id" = $2
			AND "project"."owner_id" = $3`,
		in.GetServerId(),
		in.GetProjectId(),
		in.GetUserId(),
	).Scan(&server.Password,
		&server.KeyPublic,
		&server.KeyPrivate,
		&server.KeyPassword,
		&server.Auth,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	switch server.Auth {
	case "password":
		response.Auth = serverpb.Auth_password
		response.Access = &serverpb.ServerAccess_Response_Password{
			Password: "",
		}
	case "key":
		response.Auth = serverpb.Auth_key
		response.Access = &serverpb.ServerAccess_Response_Key{
			Key: &serverpb.ServerAccess_Key{
				Public: server.KeyPublic,
			},
		}
	}

	return response, nil
}

// UpdateServerAccess is updates an affordable option for connecting to the server
func (s *server) UpdateServerAccess(ctx context.Context, in *serverpb.UpdateServerAccess_Request) (*serverpb.UpdateServerAccess_Response, error) {
	if !isOwnerProject(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var sqlQuery string
	response := new(serverpb.UpdateServerAccess_Response)

	switch in.Auth {
	case serverpb.Auth_password:
		sqlQuery, _ = sanitize.SQL(`UPDATE "server" SET "password" = $3 WHERE "id" = $1 AND "project_id" = $2`,
			in.GetServerId(),
			in.GetProjectId(),
			in.GetPassword(),
		)
	case serverpb.Auth_key:
		privateKey, err := service.cache.Get(fmt.Sprintf("tmp_key_ssh::%s", in.GetKeyUuid()))
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errServerError
		}
		service.cache.Delete(fmt.Sprintf("tmp_key_ssh::%s", in.GetKeyUuid()))
		sqlQuery, _ = sanitize.SQL(`UPDATE "server" SET "public_key" = $3, "private_key" = $4 WHERE "id" = $1 AND "project_id" = $2`,
			in.GetServerId(),
			in.GetProjectId(),
			in.GetPublicKey(),
			privateKey,
		)
	default:
		return response, nil
	}

	data, err := service.db.Conn.Exec(sqlQuery)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// ServerActivity is ...
func (s *server) ServerActivity(ctx context.Context, in *serverpb.ServerActivity_Request) (*serverpb.ServerActivity_Response, error) {
	if !isOwnerProject(in.GetProjectId(), in.GetUserId()) {
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

	rows, err := service.db.Conn.Query(`SELECT
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
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	for rows.Next() {
		day := dayActive{}
		if err := rows.Scan(&day.activityID, &day.week, &day.hour); err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			service.log.FromGRPC(err).Send()
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

// UpdateServerActivity
func (s *server) UpdateServerActivity(ctx context.Context, in *serverpb.UpdateServerActivity_Request) (*serverpb.UpdateServerActivity_Response, error) {
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

	oldActivity, err := s.ServerActivity(ctx, &serverpb.ServerActivity_Request{
		UserId:    in.GetUserId(),
		ServerId:  in.GetServerId(),
		ProjectId: in.GetProjectId(),
	})
	if err != nil {
		service.log.FromGRPC(err).Send()
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
		data, err := service.db.Conn.Exec(sqlQuery["del"])
		if err != nil {
			service.log.FromGRPC(err).Send()
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
		data, err := service.db.Conn.Exec(sqlQuery["add"])
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToAdd
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}
	}

	return response, nil
}

// ListShareServers is ...
func (s *server) ListShareServers(ctx context.Context, in *serverpb.ListShareServers_Request) (*serverpb.ListShareServers_Response, error) {
	response := new(serverpb.ListShareServers_Response)

	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := service.db.Conn.Query(`SELECT
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
		service.log.FromGRPC(err).Send()
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
			service.log.FromGRPC(err).Send()
			return nil, errServerError
		}
		server.ProjectLogin = projectLogin

		response.Servers = append(response.Servers, server)
	}
	defer rows.Close()

	// Total count for pagination
	err = service.db.Conn.QueryRow(`SELECT COUNT(*)
		FROM "server"
			INNER JOIN "project_member" ON "server"."project_id" = "project_member"."project_id"
		WHERE "project_member"."user_id" = $1`,
		in.GetUserId(),
	).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	return response, nil
}

// TODO AddShareServer is ...
func (s *server) AddShareServer(ctx context.Context, in *serverpb.AddShareServer_Request) (*serverpb.AddShareServer_Response, error) {
	response := new(serverpb.AddShareServer_Response)
	return response, nil
}

// TODO UpdateShareServer is ...
func (s *server) UpdateShareServer(ctx context.Context, in *serverpb.UpdateShareServer_Request) (*serverpb.UpdateShareServer_Response, error) {
	response := new(serverpb.UpdateShareServer_Response)
	return response, nil
}

// TODO DeleteShareServer is ...
func (s *server) DeleteShareServer(ctx context.Context, in *serverpb.DeleteShareServer_Request) (*serverpb.DeleteShareServer_Response, error) {
	response := new(serverpb.DeleteShareServer_Response)
	return response, nil
}

// UpdateHostKey is ...
func (s *server) UpdateHostKey(ctx context.Context, in *serverpb.UpdateHostKey_Request) (*serverpb.UpdateHostKey_Response, error) {
	response := new(serverpb.UpdateHostKey_Response)

	data, err := service.db.Conn.Exec(`UPDATE "server_host_key" SET "host_key" = $1 WHERE "server_id" = $2`,
		in.GetHostkey(),
		in.GetServerId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// AddSession is ...
func (s *server) AddSession(ctx context.Context, in *serverpb.AddSession_Request) (*serverpb.AddSession_Response, error) {
	response := new(serverpb.AddSession_Response)

	err := service.db.Conn.QueryRow(`INSERT INTO "session" ("account_id", "status", "created", "message")
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		in.GetAccountId(),
		strings.ToLower(in.Status.String()),
		time.Now(),
		in.GetMessage(),
	).Scan(&response.SessionId)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	return response, nil
}

// ServerNameByID is ...
func (s *server) ServerNameByID(ctx context.Context, in *serverpb.ServerNameByID_Request) (*serverpb.ServerNameByID_Response, error) {
	if !isOwnerProject(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	response := new(serverpb.ServerNameByID_Response)

	err := service.db.Conn.QueryRow(`SELECT "title" FROM "server" WHERE "id" = $1`,
		in.GetServerId(),
	).Scan(&response.ServerName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	return response, nil
}
