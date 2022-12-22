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

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/crypto"
	"github.com/werbot/werbot/internal/utils/parse"

	pb_server "github.com/werbot/werbot/api/proto/server"
)

type server struct {
	pb_server.UnimplementedServerHandlersServer
}

// TODO: a method for updating Host FingerPrint server
// TODO: When updating the IP address of the server, you need to update HostKey !!!!

// GetServers is ...
// TODO: Add a check of working hours (SERVER_ACTivity table)
// TODO: It is necessary that the server remains in the global variable Serverlist
func (s *server) ListServers(ctx context.Context, in *pb_server.ListServers_Request) (*pb_server.ListServers_Response, error) {
	// TODO: checking the permitted servers available for display
	//  log.Info().Msgf("Expired: %v", license.L.GetCustomer())
	var count int32
	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	query := service.db.QueryParse(in.GetQuery())
	servers := []*pb_server.Server_Response{}

	if query["user_name"] != "" {
		nameArray := parse.Username(query["user_name"])
		nameLen := len(nameArray)
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
      FROM
        "user"
        JOIN "project_member" ON "user"."id" = "project_member"."user_id"
        JOIN "project" ON "project"."id" = "project_member"."project_id"
        JOIN "server" ON "project"."id" = "server"."project_id"
        JOIN "server_host_key" ON "server_host_key"."server_id" = "server"."id"
        JOIN "server_member" ON "server_member"."server_id" = "server"."id"
        AND "server_member"."member_id" = "project_member"."id"
      WHERE
        "user"."name" = $1
        AND "server_member"."active" = TRUE
        AND "server"."active" = TRUE`

		switch nameLen {
		case 1:
			rows, err := service.db.Conn.Query(query, nameArray[0])
			if err != nil {
				service.log.ErrorGRPC(err)
				return nil, errFailedToSelect
			}

			for rows.Next() {
				server := new(pb_server.Server_Response)
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
					service.log.ErrorGRPC(err)
					return nil, errFailedToScan
				}
				servers = append(servers, server)
			}
			defer rows.Close()

		case 2:
			rows, err := service.db.Conn.Query(query+` AND "project"."login" = $2`,
				nameArray[0],
				nameArray[1],
			)
			if err != nil {
				service.log.ErrorGRPC(err)
				return nil, errFailedToSelect
			}

			for rows.Next() {
				server := new(pb_server.Server_Response)
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
					service.log.ErrorGRPC(err)
					return nil, errFailedToScan
				}
				servers = append(servers, server)
			}
			defer rows.Close()

		case 3:
			rows, err := service.db.Conn.Query(query+` AND "project"."login" = $2
				AND "token" = $3
				AND "project_member"."role" = 'user'`,
				nameArray[0],
				nameArray[1],
				nameArray[2],
			)
			if err != nil {
				service.log.ErrorGRPC(err)
				return nil, errFailedToSelect
			}

			for rows.Next() {
				server := new(pb_server.Server_Response)
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
					service.log.ErrorGRPC(err)
					return nil, errFailedToScan
				}
				servers = append(servers, server)
			}
			defer rows.Close()
		}

		count = int32(len(servers))
		if count == 0 {
			return nil, errNotFound
		}

		return &pb_server.ListServers_Response{
			Total:   count,
			Servers: servers,
		}, nil
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
				( SELECT COUNT ( * ) FROM "server_member" WHERE "server_id" = "server"."id"  ) AS "count_members"
			FROM
				"server"
				INNER JOIN "project" ON "server"."project_id" = "project"."id"
			WHERE
				"server"."project_id" = $1
				AND "project"."owner_id" = $2`+sqlFooter,
			query["project_id"],
			query["user_id"],
		)
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, errFailedToSelect
		}

		for rows.Next() {
			server := new(pb_server.Server_Response)
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
				service.log.ErrorGRPC(err)
				return nil, errFailedToScan
			}
			servers = append(servers, server)
		}
		defer rows.Close()

		// Total count
		err = service.db.Conn.QueryRow(`SELECT
        COUNT (*)
      FROM
				"server"
				INNER JOIN "project" ON "server"."project_id" = "project"."id"
			WHERE
				"server"."project_id" = $1
				AND "project"."owner_id" = $2`,
			query["project_id"],
			query["user_id"],
		).Scan(&count)
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, errFailedToScan
		}

		return &pb_server.ListServers_Response{
			Total:   count,
			Servers: servers,
		}, nil
	}

	return &pb_server.ListServers_Response{}, nil
}

// Server is ...
func (s *server) Server(ctx context.Context, in *pb_server.Server_Request) (*pb_server.Server_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var privateDescription, publicDescription pgtype.Text
	server := new(pb_server.Server_Response)
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
		FROM
			"server"
		WHERE
			"server"."id" = $1
			AND "server"."project_id" = $2`,
		in.GetServerId(),
		in.GetProjectId(),
	).Scan(
		&server.Address,
		&server.Port,
		&server.Token,
		&server.Login,
		&privateDescription,
		&publicDescription,
		&server.Title,
		&server.Active,
		&server.Audit,
		&server.Online,
		&server.Auth,
		&server.Scheme,
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToScan
	}

	server.PrivateDescription = privateDescription.String
	server.PublicDescription = publicDescription.String
	return server, nil
}

// DeleteServer is ...
func (s *server) DeleteServer(ctx context.Context, in *pb_server.DeleteServer_Request) (*pb_server.DeleteServer_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	data, err := service.db.Conn.Exec(`DELETE
		FROM
			"server"
		WHERE
			"id" = $1
			AND "project_id" = $2`,
		in.GetServerId(),
		in.GetProjectId(),
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_server.DeleteServer_Response{}, nil
}

// ServerAccess is ...
func (s *server) ServerAccess(ctx context.Context, in *pb_server.ServerAccess_Request) (*pb_server.ServerAccess_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	access := new(pb_server.ServerAccess_Response)
	server := new(pb_server.Server_Response)
	err := service.db.Conn.QueryRowx(`SELECT
			"server"."password",
			"server"."public_key",
			"server"."private_key",
			"server"."private_key_password",
			"server"."auth"
		FROM
			"server"
			INNER JOIN "project" ON "server"."project_id" = "project". "id"
		WHERE
			"server"."id" = $1
			AND "server"."project_id" = $2
			AND "project"."owner_id" = $3`,
		in.GetServerId(),
		in.GetProjectId(),
		in.GetUserId(),
	).Scan(
		&server.Password,
		&server.KeyPublic,
		&server.KeyPrivate,
		&server.KeyPassword,
		&server.Auth,
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToSelect
	}

	switch server.Auth {
	case "password":
		access.Auth = pb_server.ServerAuth_PASSWORD
		// access.Password = data.Password
		access.Password = ""
	case "key":
		access.Auth = pb_server.ServerAuth_KEY
		access.PublicKey = server.KeyPublic
		// access.PrivateKey = data.KeyPrivate
		// access.PasswordKey = data.KeyPassword
	}

	return access, nil
}

// UpdateServerAccess is ...
func (s *server) UpdateServerAccess(ctx context.Context, in *pb_server.UpdateServerAccess_Request) (*pb_server.UpdateServerAccess_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var err error
	var data sql.Result

	switch in.Auth {
	case pb_server.ServerAuth_PASSWORD:
		data, err = service.db.Conn.Exec(`UPDATE
        "server"
			SET
				"password" = $3
			WHERE
				"id" = $1
				AND "project_id" = $2`,
			in.GetServerId(),
			in.GetProjectId(),
			in.GetPassword(),
		)
	case pb_server.ServerAuth_KEY:
		privateKey, _err := service.cache.Get(fmt.Sprintf("tmp_key_ssh::%s", in.GetKeyUuid()))
		if _err != nil {
			service.log.ErrorGRPC(err)
			return nil, errFailedToSelect
		}
		service.cache.Delete(fmt.Sprintf("tmp_key_ssh::%s", in.GetKeyUuid()))

		data, err = service.db.Conn.Exec(`UPDATE
        "server"
			SET
				"public_key" = $3,
				"private_key" = $4
			WHERE
				"id" = $1
				AND "project_id" = $2`,
			in.GetServerId(),
			in.GetProjectId(),
			in.GetPublicKey(),
			privateKey,
		)
	}

	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_server.UpdateServerAccess_Response{}, nil
}

// UpdateServerOnlineStatus is ...
func (s *server) UpdateServerOnlineStatus(ctx context.Context, in *pb_server.UpdateServerOnlineStatus_Request) (*pb_server.UpdateServerOnlineStatus_Response, error) {
	data, err := service.db.Conn.Exec(`UPDATE
      "server"
		SET
			"online" = $1
		FROM
			"project"
		WHERE
			"server"."id" = $2 AND
			"project"."owner_id"  = $3 AND
			"server"."project_id" = "project"."id"`,
		in.GetStatus(),
		in.GetServerId(),
		in.GetUserId(),
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_server.UpdateServerOnlineStatus_Response{}, nil
}

// UpdateServerActiveStatus is ...
func (s *server) UpdateServerActiveStatus(ctx context.Context, in *pb_server.UpdateServerActiveStatus_Request) (*pb_server.UpdateServerActiveStatus_Response, error) {
	// TODO After turning off, turn off all users who online
	data, err := service.db.Conn.Exec(`UPDATE
      "server"
		SET
			"active" = $1
		FROM
			"project"
		WHERE
			"server"."id" = $2 AND
			"project"."owner_id"  = $3 AND
			"server"."project_id" = "project"."id"`,
		in.GetStatus(),
		in.GetServerId(),
		in.GetUserId(),
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_server.UpdateServerActiveStatus_Response{}, nil
}

// UpdateServerHostKey is ...
func (s *server) UpdateServerHostKey(ctx context.Context, in *pb_server.UpdateServerHostKey_Request) (*pb_server.UpdateServerHostKey_Response, error) {
	data, err := service.db.Conn.Exec(`UPDATE
      "server_host_key"
		SET
			"host_key" = $1
		WHERE
			"server_id" = $2`,
		in.GetHostkey(),
		in.GetServerId(),
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_server.UpdateServerHostKey_Response{}, nil
}

// CreateServerSession is ...
func (s *server) CreateServerSession(ctx context.Context, in *pb_server.CreateServerSession_Request) (*pb_server.CreateServerSession_Response, error) {
	if in.GetAccountId() == "" && in.GetUuid() == "" {
		return nil, errBadRequest
	}

	var sessionID string
	err := service.db.Conn.QueryRow(`INSERT
		INTO "session" (
			"account_id",
			"status",
			"created",
			"message",
			"uuid")
		VALUES
			($1, $2, $3, $4, $5)
		RETURNING id`,
		in.GetAccountId(),
		strings.ToLower(in.Status.String()),
		time.Now(),
		in.GetMessage(),
		in.GetUuid(),
	).Scan(&sessionID)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToAdd
	}

	return &pb_server.CreateServerSession_Response{
		SessionId: sessionID,
	}, nil
}

// CreateServer is ...
func (s *server) CreateServer(ctx context.Context, in *pb_server.CreateServer_Request) (*pb_server.CreateServer_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var serverPassword string
	var serverKeys = new(crypto.PairOfKeys)
	var err error

	switch in.GetAuth() {
	case pb_server.ServerAuth_PASSWORD:
		serverPassword = in.GetPassword()
	case pb_server.ServerAuth_KEY:
		if in.GetPublicKey() != "" && in.GetKeyUuid() != "" {
			privateKey, err := service.cache.Get(fmt.Sprintf("tmp_key_ssh::%s", in.GetKeyUuid()))
			if err != nil {
				service.log.ErrorGRPC(err)
				return nil, errFailedToSelect
			}
			service.cache.Delete(fmt.Sprintf("tmp_key_ssh::%s", in.GetKeyUuid()))
			serverKeys.PrivateKey = []byte(privateKey)
			serverKeys.PublicKey = []byte(in.PublicKey)
		} else {
			serverKeys, err = crypto.NewSSHKey(internal.GetString("SECURITY_SSH_KEY_TYPE", "KEY_TYPE_ED25519"))
			if err != nil {
				service.log.ErrorGRPC(err)
				return nil, crypto.ErrFailedCreatingSSHKey
			}
		}
	}

	serverToken := crypto.NewPassword(6, false)
	serverTitle := in.GetTitle()
	if in.Title == "" {
		serverTitle = fmt.Sprintf("server #%s", serverToken)
	}

	tx, err := service.db.Conn.Beginx()
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errTransactionCreateError
	}

	// TODO: This design converts the number into a line into an old format that is registered in the database,
	// I recommend that you store numerical values in the new format in the database
	serverAuth := strings.ToLower(pb_server.ServerAuth_name[int32(in.Auth.Number())])
	serverScheme := strings.ToLower(pb_server.ServerScheme_name[int32(in.Scheme.Number())])

	var serverID string
	err = tx.QueryRow(`INSERT
		INTO "server" (
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
			"private_key_password"
		)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, '[]', $18)
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
	).Scan(&serverID)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToScan
	}

	// + record server_access_policy
	data, err := tx.Exec(`INSERT
		INTO "server_access_policy" (
			"server_id",
			"ip",
			"country"
		)
		VALUES
			($1, 'f', 'f')`,
		&serverID,
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToAdd
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	// + record server_activity
	sqlCountDay := `INSERT
		INTO "server_activity" (
			"server_id",
			"dow",
			"time_from",
			"time_to"
		)
		VALUES `
	for countDay := 1; countDay < 8; countDay++ {
		for countHour := 0; countHour < 24; countHour++ {
			timeFrom := fmt.Sprintf("%02v:00:00", strconv.Itoa(countHour))
			timeTo := fmt.Sprintf("%02v:59:59", strconv.Itoa(countHour))
			sqlCountDay += fmt.Sprintf(`(%v, %v, '%v', '%v'),`, serverID, countDay, timeFrom, timeTo)
		}
	}
	sqlCountDay = strings.TrimSuffix(sqlCountDay, ",")
	data, err = tx.Exec(sqlCountDay)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToAdd
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	if err = tx.Commit(); err != nil {
		service.log.ErrorGRPC(err)
		return nil, errTransactionCommitError
	}

	return &pb_server.CreateServer_Response{
		ServerId:  serverID,
		KeyPublic: string(serverKeys.PublicKey),
	}, nil
}

// UpdateServer is ...
func (s *server) UpdateServer(ctx context.Context, in *pb_server.UpdateServer_Request) (*pb_server.UpdateServer_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	data, err := service.db.Conn.Exec(`UPDATE "server"
		SET
			"address" = $3,
			"port" = $4,
			"login" = $5,
			"title" = $6,
			"active" = $7,
			"audit" = $8,
			"private_description" = $9,
			"public_description" = $10
		WHERE
			"id" = $1
			AND "project_id" = $2`,
		in.GetServerId(),
		in.GetProjectId(),
		in.GetAddress(),
		in.GetPort(),
		in.GetLogin(),
		in.GetTitle(),
		in.GetActive(),
		in.GetAudit(),
		in.GetPrivateDescription(),
		in.GetPublicDescription(),
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_server.UpdateServer_Response{}, nil
}

// ServerActivity is ...
func (s *server) ServerActivity(ctx context.Context, in *pb_server.ServerActivity_Request) (*pb_server.ServerActivity_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	data := []map[string]int32{}
	rows, err := service.db.Conn.Query(`SELECT
			"server_activity"."id" AS "activity_id",
			"server_activity"."dow" AS "week",
			EXTRACT ( HOUR FROM "server_activity"."time_from" ) AS "hour"
		FROM
			"server_activity"
			INNER JOIN "server" ON "server_activity"."server_id" = "server"."id"
		WHERE
			"server_activity"."server_id" = $1
			AND "server"."project_id" = $2`,
		in.GetServerId(),
		in.GetProjectId(),
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToSelect
	}

	for rows.Next() {
		var activityID, week, hour int32
		err = rows.Scan(
			&activityID,
			&week,
			&hour,
		)
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, errFailedToScan
		}
		data = append(data, map[string]int32{
			"activity_id": activityID,
			"week":        week,
			"hour":        hour,
		})
	}
	defer rows.Close()

	activity := &pb_server.ServerActivity_Response{
		Monday:    make([]int32, 24),
		Tuesday:   make([]int32, 24),
		Wednesday: make([]int32, 24),
		Thursday:  make([]int32, 24),
		Friday:    make([]int32, 24),
		Saturday:  make([]int32, 24),
		Sunday:    make([]int32, 24),
	}

	for _, item := range data {
		var status int32
		if item["activity_id"] > 0 {
			status = 1
		}

		switch item["week"] {
		case 1:
			activity.Monday[item["hour"]] = status
		case 2:
			activity.Tuesday[item["hour"]] = status
		case 3:
			activity.Wednesday[item["hour"]] = status
		case 4:
			activity.Thursday[item["hour"]] = status
		case 5:
			activity.Friday[item["hour"]] = status
		case 6:
			activity.Saturday[item["hour"]] = status
		case 7:
			activity.Sunday[item["hour"]] = status
		}
	}

	return activity, nil
}

// UpdateServerActivity
func (s *server) UpdateServerActivity(ctx context.Context, in *pb_server.UpdateServerActivity_Request) (*pb_server.UpdateServerActivity_Response, error) {
	var sqlDelete, sqlInsert string
	week := map[int32]string{
		1: "Monday",
		2: "Tuesday",
		3: "Wednesday",
		4: "Thursday",
		5: "Friday",
		6: "Saturday",
		7: "Sunday",
	}

	oldActivity, err := s.ServerActivity(ctx, &pb_server.ServerActivity_Request{
		UserId:    in.GetUserId(),
		ServerId:  in.GetServerId(),
		ProjectId: in.GetProjectId(),
	})
	if err != nil {
		service.log.ErrorGRPC(err)
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
					sqlDelete += fmt.Sprintf(` ("server_id" = %v AND "dow" = %v AND "time_from" = '%v:00:00') OR`, in.GetServerId(), index, hour)
				} else {
					sqlInsert += fmt.Sprintf(` (%v, %v, '%v:00:00', '%v:59:59'),`, in.GetServerId(), index, hour, hour)
				}
			}
		}
	}

	if sqlDelete != "" {
		sqlDelete = fmt.Sprintf(`DELETE
			FROM
				"server_activity"
			WHERE
				%s`,
			sqlDelete[:len(sqlDelete)-2],
		)
		data, err := service.db.Conn.Exec(sqlDelete)
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, errFailedToDelete
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}
	}

	if sqlInsert != "" {
		sqlInsert = fmt.Sprintf(`INSERT
			INTO "server_activity" (
				"server_id",
				"dow",
				"time_from",
				"time_to"
			)
			VALUES
				%s`,
			sqlInsert[:len(sqlInsert)-1],
		)
		data, err := service.db.Conn.Exec(sqlInsert)
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, errFailedToAdd
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}
	}

	return &pb_server.UpdateServerActivity_Response{}, nil
}

// ServerNameByID is ...
func (s *server) ServerNameByID(ctx context.Context, in *pb_server.ServerNameByID_Request) (*pb_server.ServerNameByID_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var name string
	err := service.db.Conn.QueryRow(`SELECT
			"title"
		FROM
			"server"
		WHERE
			"id" = $1`,
		in.GetServerId(),
	).Scan(&name)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToScan
	}

	return &pb_server.ServerNameByID_Response{
		ServerName: name,
	}, nil
}

// ListServersShareForUser is ...
func (s *server) ListServersShareForUser(ctx context.Context, in *pb_server.ListServersShareForUser_Request) (*pb_server.ListServersShareForUser_Response, error) {
	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	servers := []*pb_server.ListServersShareForUser_Response_SharedServer{}
	rows, err := service.db.Conn.Query(`SELECT
			"user"."name" AS user_login,
			"project"."login" AS project_login,
			"project"."title" AS project_title,
			"server"."token" AS server_token,
			"server"."id" AS server_id,
			"server"."online" AS server_online,
			"server"."title" AS server_title,
			"server"."public_description" AS server_description
		FROM
			"server"
			INNER JOIN "project" ON "server"."project_id" = "project"."id"
			INNER JOIN "project_member" ON "project"."id" = "project_member"."project_id"
			INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
		WHERE
			"project_member"."user_id" = $1`+sqlFooter,
		in.UserId,
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToSelect
	}

	for rows.Next() {
		var projectLogin, projectTitle string
		server := new(pb_server.ListServersShareForUser_Response_SharedServer)
		err = rows.Scan(
			&server.UserLogin,
			&projectLogin,
			&projectTitle,
			&server.ServerToken,
			&server.ServerId,
			&server.ServerOnline,
			&server.ServerTitle,
			&server.ServerDescription,
		)
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, errFailedToScan
		}
		server.ProjectLogin = projectLogin
		servers = append(servers, server)
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	err = service.db.Conn.QueryRow(`SELECT
      COUNT(*)
		FROM
			"server"
			INNER JOIN "project_member" ON "server"."project_id" = "project_member"."project_id"
		WHERE
			"project_member"."user_id" = $1`,
		in.GetUserId(),
	).Scan(&total)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToScan
	}

	return &pb_server.ListServersShareForUser_Response{
		Total:   total,
		Servers: servers,
	}, nil
}
