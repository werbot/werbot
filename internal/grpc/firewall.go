package grpc

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/oschwald/geoip2-golang"

	"github.com/werbot/werbot/internal/config"
	"github.com/werbot/werbot/internal/message"
	"github.com/werbot/werbot/internal/utils/convert"

	pb_firewall "github.com/werbot/werbot/internal/grpc/proto/firewall"
)

type firewall struct {
	pb_firewall.UnimplementedFirewallHandlersServer
}

func (s *firewall) getAccessList(serverID string) (*pb_firewall.AccessList, error) {
	accessList := pb_firewall.AccessList{}
	row := db.Conn.QueryRowx(`SELECT
			"server_access_policy"."ip",
			"server_access_policy"."country" 
		FROM
			"server_access_policy" 
		WHERE
			"server_access_policy"."server_id" = $1`,
		serverID,
	)
	if err := row.Scan(&accessList.Network, &accessList.Country); err != nil {
		return nil, err
	}

	return &accessList, nil
}

// GetServerFirewall is ...
func (s *firewall) GetServerFirewall(ctx context.Context, in *pb_firewall.GetServerFirewall_Request) (*pb_firewall.GetServerFirewall_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetUserId()) {
		return nil, errors.New(message.ErrNotFound)
	}

	// get countries
	countries := []*pb_firewall.Country{}
	rows, err := db.Conn.Query(`SELECT
			"server_security_country"."id",
			"server_security_country"."country_code",
			"country"."name"
		FROM
			"server_security_country"
			INNER JOIN "country" ON "server_security_country"."country_code" = "country"."code"
		WHERE
			"server_security_country"."server_id" = $1`,
		in.GetServerId(),
	)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		country := pb_firewall.Country{}
		err = rows.Scan(
			&country.Id,
			&country.CountryCode,
			&country.CountryName,
		)

		if err != nil {
			return nil, err
		}

		countries = append(countries, &country)
	}
	defer rows.Close()

	// get networks
	networks := []*pb_firewall.Network{}
	rows, err = db.Conn.Query(`SELECT
			"id",
			"start_ip",
			"end_ip" 
		FROM
			"server_security_ip" 
		WHERE
			"server_id" = $1`,
		in.GetServerId(),
	)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		network := pb_firewall.Network{}
		err = rows.Scan(
			&network.Id,
			&network.StartIp,
			&network.EndIp,
		)

		if err != nil {
			return nil, err
		}

		networks = append(networks, &network)
	}
	defer rows.Close()

	// get status black lists
	accessList, err := s.getAccessList(in.GetServerId())
	if err != nil {
		return nil, errors.New("server_access_policy failed")
	}

	return &pb_firewall.GetServerFirewall_Response{
		Country: &pb_firewall.GetServerFirewall_Response_Countries{
			WiteList: accessList.Country,
			List:     countries,
		},
		Network: &pb_firewall.GetServerFirewall_Response_Networks{
			WiteList: accessList.Network,
			List:     networks,
		},
	}, nil
}

// CreateServerFirewall is ...
func (s *firewall) CreateServerFirewall(ctx context.Context, in *pb_firewall.CreateServerFirewall_Request) (*pb_firewall.CreateServerFirewall_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetUserId()) {
		return nil, errors.New(message.ErrNotFound)
	}

	response := &pb_firewall.CreateServerFirewall_Response{}

	var recordID string
	var err error
	switch record := in.Record.(type) {
	case *pb_firewall.CreateServerFirewall_Request_Country:
		row := db.Conn.QueryRowx(`SELECT 
				"id" 
			FROM 
				"server_security_country" 
			WHERE 
				"server_id" = $1 
				AND "country_code" = $2`,
			in.GetServerId(),
			record.Country.Code,
		)
		row.Scan(&recordID)
		if recordID != "" {
			return nil, errors.New(message.MsgObjectAlreadyExists)
		}

		err = db.Conn.QueryRow(`INSERT 
			INTO "server_security_country" (
				"server_id", 
				"country_code") 
			VALUES 
				($1, $2) 
			RETURNING id`,
			in.GetServerId(),
			record.Country.Code,
		).Scan(&recordID)
		if err != nil {
			return nil, err
		}

		response.Id = recordID

	case *pb_firewall.CreateServerFirewall_Request_Ip:
		row := db.Conn.QueryRowx(`SELECT 
				"id" 
			FROM 
				"server_security_ip" 
			WHERE 
				"server_id" = $1 
				AND "start_ip" = $2 
				AND "end_ip" = $3`,
			in.GetServerId(),
			record.Ip.StartIp,
			record.Ip.EndIp,
		)
		row.Scan(&recordID)
		if recordID != "" {
			return nil, errors.New(message.MsgObjectAlreadyExists)
		}

		err = db.Conn.QueryRow(`INSERT 
			INTO "server_security_ip" (
				"server_id", 
				"start_ip", 
				"end_ip") 
			VALUES 
				($1, $2, $3) 
			RETURNING id`,
			in.GetServerId(),
			record.Ip.StartIp,
			record.Ip.EndIp,
		).Scan(&recordID)
		if err != nil {
			return nil, err
		}

		response.Id = recordID

	default:
		return response, nil
	}

	return response, nil
}

// UpdateAccessPolicy is ...
func (s *firewall) UpdateAccessPolicy(ctx context.Context, in *pb_firewall.UpdateAccessPolicy_Request) (*pb_firewall.UpdateAccessPolicy_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetUserId()) {
		return nil, errors.New(message.ErrNotFound)
	}

	var sql string
	switch in.Rule {
	case pb_firewall.Rules_country:
		sql = `UPDATE "server_access_policy" 
			SET 
				"country" = $1 
			WHERE 
				"server_id" = $2`
	case pb_firewall.Rules_ip:
		sql = `UPDATE "server_access_policy" 
			SET 
				"ip" = $1 
			WHERE 
				"server_id" = $2`
	default:
		return &pb_firewall.UpdateAccessPolicy_Response{}, nil
	}

	if _, err := db.Conn.Exec(sql, in.GetStatus(), in.GetServerId()); err != nil {
		return nil, err
	}

	return &pb_firewall.UpdateAccessPolicy_Response{}, nil
}

// DeleteServerFirewall is ...
func (s *firewall) DeleteServerFirewall(ctx context.Context, in *pb_firewall.DeleteServerFirewall_Request) (*pb_firewall.DeleteServerFirewall_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetUserId()) {
		return nil, errors.New(message.ErrNotFound)
	}

	var sql string
	switch in.Rule {
	case pb_firewall.Rules_country:
		sql = `DELETE 
			FROM 
				"server_security_country" 
			WHERE 
				"id" = $1`
	case pb_firewall.Rules_ip:
		sql = `DELETE 
			FROM 
				"server_security_ip" 
			WHERE 
				"id" = $1`
	default:
		return &pb_firewall.DeleteServerFirewall_Response{}, nil
	}

	if _, err := db.Conn.Exec(sql, in.GetRecordId()); err != nil {
		return nil, err
	}

	return &pb_firewall.DeleteServerFirewall_Response{}, nil
}

// старая версия https://git.piplos.by/werbot/werbot-server/-/blob/feature/audit-record/pkg/acl/security.go
// https://git.piplos.by/werbot/old-werbot/-/blob/master/wserver/firewall.go

// CheckIPAccess is ...
func (s *firewall) CheckIPAccess(ctx context.Context, in *pb_firewall.CheckIPAccess_Request) (*pb_firewall.CheckIPAccess_Response, error) {
	if in.GetClientIp() == "127.0.0.1" {
		return &pb_firewall.CheckIPAccess_Response{
			Access: true,
		}, nil
	}

	if in.GetClientIp() == "" {
		return &pb_firewall.CheckIPAccess_Response{
			Access: false,
		}, errors.New("Incorrect parameters")
	}

	// проверка страны по глобальному списку запрещенных стран
	countryCode, _ := countryFromIP(in.GetClientIp())
	if convert.StringInSlice(*countryCode, config.GetSliceString("SECURITY_BAD_COUNTRY", "")) {
		return &pb_firewall.CheckIPAccess_Response{
			Access: false,
		}, nil
	}

	// проверка ip по глобальному списку
	access, err := blackListIP(in.GetClientIp())
	if err != nil {
		return &pb_firewall.CheckIPAccess_Response{
			Access: false,
		}, errors.New("blackList IP failed")
	}

	if access {
		return &pb_firewall.CheckIPAccess_Response{
			Access: false,
		}, nil
	}

	return &pb_firewall.CheckIPAccess_Response{
		Access:  true,
		Country: *countryCode,
	}, nil
}

// CheckServerAccess is ...
func (s *firewall) CheckServerAccess(ctx context.Context, in *pb_firewall.CheckServerAccess_Request) (*pb_firewall.CheckServerAccess_Response, error) {
	// включен или нет профиль
	access, err := userAccountActivity(in.GetAccountId(), in.GetUserId())
	if err != nil {
		return &pb_firewall.CheckServerAccess_Response{
			Access: false,
		}, errors.New("user Account Activity failed")
	}

	// проверка страны и ip адреса по black и white листам
	if access {
		access, err = accountActivityList(in.GetAccountId(), in.GetCountry(), in.GetClientIp())
		if err != nil {
			return &pb_firewall.CheckServerAccess_Response{
				Access: false,
			}, errors.New("account Activity List failed")
		}
	}

	// проверить по времени
	if access {
		access, err = timeAccountActivity(in.GetAccountId())
		if err != nil {
			return &pb_firewall.CheckServerAccess_Response{
				Access: false,
			}, errors.New("time Account Activity failed")
		}
	}

	return &pb_firewall.CheckServerAccess_Response{
		Access: access,
	}, nil
}

func countryFromIP(ip string) (*string, error) {
	db, err := geoip2.Open(config.GetString("SECURITY_GEOIP2", "/etc/geoip2/GeoLite2-Country.mmdb"))
	if err != nil {
		return nil, errors.New("countryFromIP failed")
	}
	defer db.Close()

	record, err := db.City(net.ParseIP(ip))
	if err != nil {
		return nil, errors.New("countryFromIP city failed")
	}
	return &record.Country.IsoCode, nil
}

// timeAccountActivity is ...
func timeAccountActivity(accountID string) (bool, error) {
	id := 0
	access := true
	weekDays := [...]int32{7, 1, 2, 3, 4, 5, 6}
	nowWeekInt := weekDays[time.Now().Weekday()]
	nowTime := time.Now().Local().Format("15:04:05")

	row := db.Conn.QueryRow(`SELECT
			"server_activity"."id" 
		FROM
			"server_activity" 
		WHERE
			"server_activity"."server_id" = $1 
			AND "server_activity"."dow" = $2 
			AND "server_activity"."time_from" < $3 
			AND "server_activity"."time_to" > $3`,
		accountID,
		nowWeekInt,
		nowTime,
	)
	if err := row.Scan(&id); err != nil {
		return false, errors.New("select to server_activity")
	}
	if id == 0 {
		return false, errors.New("Access to this server is blocked at this time")
	}
	return access, nil
}

// accountActivityList is ...
// accessListCountry и accessListIP может возвращать следующие значения:
//
//	true - у данного сервера активен белый список
//	false - у данного сервера активен черный список
func accountActivityList(accountID, country, ip string) (bool, error) {
	if ip == "127.0.0.1" {
		return true, nil
	}

	var accessListCountry, accessListIP bool
	row := db.Conn.QueryRow(`SELECT
			"server_access_policy"."ip",
			"server_access_policy"."country" 
		FROM
			"server_access_policy" 
		WHERE
			"server_access_policy"."server_id" =$1`,
		accountID,
	)
	if err := row.Scan(&accessListIP, &accessListCountry); err != nil {
		return false, errors.New("server_access_policy failed")
	}

	// выборка из таблицы со странами
	count := 0
	row = db.Conn.QueryRow(`SELECT COUNT (*) 
		FROM 
			"server_access_policy" 
		INNER 
			JOIN "server_security_country" ON "server_access_policy"."server_id"="server_security_country"."server_id" 
		WHERE 
			"server_access_policy"."server_id" = $1 
			AND "server_security_country"."country_code" = $2`,
		accountID,
		country,
	)
	if err := row.Scan(&count); err != nil {
		return false, errors.New("server_access_policy failed")
	}

	// черный список, страна найдена в списке:
	if !accessListCountry && count > 0 {
		return false, errors.New("Access to this server is blocked at this country")
	}

	// белый список, страна не найдена в списке
	if accessListCountry && count == 0 {
		return false, errors.New("Access to this server is blocked at this country")
	}

	// делаем выборку в базе со списком ip адресов
	count = 0 // сбрасываем значение
	row = db.Conn.QueryRow(`SELECT COUNT (*)
		FROM
			"server_access_policy"
			INNER JOIN "server_security_ip" ON "server_access_policy"."server_id" = "server_security_ip"."server_id" 
		WHERE
			"server_access_policy"."server_id" = $1 
			AND $2::inet >= "server_security_ip"."start_ip" 
			AND $2::inet <= "server_security_ip"."end_ip"`,
		accountID,
		ip,
	)
	if err := row.Scan(&count); err != nil {
		return false, errors.New("select to server_security_ip")
	}

	// черный список, ip найден в списке
	if !accessListIP && count > 0 {
		return false, errors.New("Access to this server is blocked at this ip")
	}

	// белый список, ip не найден в списке
	if accessListIP && count == 0 {
		return false, errors.New("Access to this server is blocked at this ip")
	}

	return true, nil
}

// userAccountActivity is ...
func userAccountActivity(accountID, userID string) (bool, error) {
	id := 0
	row := db.Conn.QueryRow(`SELECT
			"server_member"."id" 
		FROM
			server_member
			INNER JOIN "project_member" ON "server_member"."member_id" = "project_member"."id"
			INNER JOIN "server" ON "server_member"."server_id" = "server"."id" 
		WHERE
			"project_member"."user_id" = $2 
			AND "project_member"."active" = TRUE 
			AND "server_member"."server_id" = $1 
			AND "server_member"."active" = TRUE 
			AND "server"."active" = TRUE`,
		accountID,
		userID,
	)
	if err := row.Scan(&id); err != nil {
		return false, errors.New("userAccountActivity field")
	}

	if id == 0 {
		return false, errors.New("Access is blocked")
	}
	return true, nil
}

// blackListIP is checks if the IP address is in the active blacklist
// returns true if the IP address is in the active blacklist
// returns false if the IP address is not in the active blacklist
func blackListIP(ip string) (bool, error) {
	count := 0
	row := db.Conn.QueryRow(`SELECT COUNT(*) 
		FROM
			"firewall_ip"
			INNER JOIN "firewall_list" ON "firewall_ip"."list_id" = "firewall_list"."id" 
		WHERE 
			$1::inet >= "start_ip" 
			AND $1::inet <= "end_ip"
			AND "firewall_list"."active" = TRUE`,
		ip,
	)
	if err := row.Scan(&count); err != nil {
		return true, errors.New("blackListIP failed")
	}

	// черный список, ip найден в списке
	if count > 0 {
		return true, nil
	}
	return false, nil
}
