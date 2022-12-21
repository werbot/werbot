package grpc

import (
	"context"
	"net"
	"time"

	"github.com/oschwald/geoip2-golang"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/utils/convert"

	pb_firewall "github.com/werbot/werbot/api/proto/firewall"
)

type firewall struct {
	pb_firewall.UnimplementedFirewallHandlersServer
}

func (s *firewall) getAccessList(serverID string) (*pb_firewall.AccessList, error) {
	accessList := new(pb_firewall.AccessList)
	err := service.db.Conn.QueryRowx(`SELECT
			"server_access_policy"."ip",
			"server_access_policy"."country"
		FROM
			"server_access_policy"
		WHERE
			"server_access_policy"."server_id" = $1`,
		serverID,
	).Scan(&accessList.Network, &accessList.Country)
	if err != nil {
		return nil, errFailedToSelect
	}

	return accessList, nil
}

// ServerFirewall is ...
func (s *firewall) ServerFirewall(ctx context.Context, in *pb_firewall.ServerFirewall_Request) (*pb_firewall.ServerFirewall_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	// get countries
	countries := []*pb_firewall.Country{}
	rows, err := service.db.Conn.Query(`SELECT
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
		return nil, errFailedToSelect
	}

	for rows.Next() {
		country := new(pb_firewall.Country)
		err = rows.Scan(
			&country.Id,
			&country.CountryCode,
			&country.CountryName,
		)
		if err != nil {
			return nil, errFailedToScan
		}
		countries = append(countries, country)
	}
	defer rows.Close()

	// get networks
	networks := []*pb_firewall.Network{}
	rows, err = service.db.Conn.Query(`SELECT
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
		return nil, errFailedToSelect
	}

	for rows.Next() {
		network := new(pb_firewall.Network)
		err = rows.Scan(
			&network.Id,
			&network.StartIp,
			&network.EndIp,
		)
		if err != nil {
			return nil, errFailedToScan
		}
		networks = append(networks, network)
	}
	defer rows.Close()

	// get status black lists
	accessList, err := s.getAccessList(in.GetServerId())
	if err != nil {
		return nil, err
	}

	return &pb_firewall.ServerFirewall_Response{
		Country: &pb_firewall.ServerFirewall_Response_Countries{
			WiteList: accessList.Country,
			List:     countries,
		},
		Network: &pb_firewall.ServerFirewall_Response_Networks{
			WiteList: accessList.Network,
			List:     networks,
		},
	}, nil
}

// CreateServerFirewall is ...
func (s *firewall) CreateServerFirewall(ctx context.Context, in *pb_firewall.CreateServerFirewall_Request) (*pb_firewall.CreateServerFirewall_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var recordID string
	response := new(pb_firewall.CreateServerFirewall_Response)
	switch record := in.Record.(type) {
	case *pb_firewall.CreateServerFirewall_Request_Country:
		err := service.db.Conn.QueryRowx(`SELECT
				"id"
			FROM
				"server_security_country"
			WHERE
				"server_id" = $1
				AND "country_code" = $2`,
			in.GetServerId(),
			record.Country.Code,
		).Scan(&recordID)
		if err != nil {
			return nil, errFailedToScan
		}
		if recordID != "" {
			return nil, errObjectAlreadyExists
		}

		err = service.db.Conn.QueryRow(`INSERT
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
			return nil, errFailedToAdd
		}

		response.Id = recordID

	case *pb_firewall.CreateServerFirewall_Request_Ip:
		err := service.db.Conn.QueryRowx(`SELECT
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
		).Scan(&recordID)
		if err != nil {
			return nil, errFailedToScan
		}
		if recordID != "" {
			return nil, errObjectAlreadyExists
		}

		err = service.db.Conn.QueryRow(`INSERT
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
			return nil, errFailedToAdd
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
		return nil, errNotFound
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

	data, err := service.db.Conn.Exec(sql, in.GetStatus(), in.GetServerId())
	if err != nil {
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_firewall.UpdateAccessPolicy_Response{}, nil
}

// DeleteServerFirewall is ...
func (s *firewall) DeleteServerFirewall(ctx context.Context, in *pb_firewall.DeleteServerFirewall_Request) (*pb_firewall.DeleteServerFirewall_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
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

	data, err := service.db.Conn.Exec(sql, in.GetRecordId())
	if err != nil {
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_firewall.DeleteServerFirewall_Response{}, nil
}

// old version https://git.piplos.media/werbot/werbot-server/-/blob/feature/audit-record/pkg/acl/security.go
// https://git.piplos.media/werbot/old-werbot/-/blob/master/wserver/firewall.go

// CheckIPAccess is ...
func (s *firewall) CheckIPAccess(ctx context.Context, in *pb_firewall.CheckIPAccess_Request) (*pb_firewall.CheckIPAccess_Response, error) {
	IPAccess := new(pb_firewall.CheckIPAccess_Response)
	if in.GetClientIp() == "127.0.0.1" {
		IPAccess.Access = true
		return IPAccess, nil
	}

	if in.GetClientIp() == "" {
		IPAccess.Access = false
		return IPAccess, errIncorrectParameters
	}

	// Verification of the country according to the global list of prohibited countries
	countryCode, _ := countryFromIP(in.GetClientIp())
	if convert.StringInSlice(*countryCode, internal.GetSliceString("SECURITY_BAD_COUNTRY", "")) {
		IPAccess.Access = false
		return IPAccess, nil
	}

	// porch IP on the global list
	access, err := blackListIP(in.GetClientIp())
	if err != nil || access {
		IPAccess.Access = false
		return IPAccess, errAccessIsDenied // blackList IP failed
	}

	IPAccess.Access = true
	IPAccess.Country = *countryCode
	return IPAccess, nil
}

// CheckServerAccess is ...
func (s *firewall) CheckServerAccess(ctx context.Context, in *pb_firewall.CheckServerAccess_Request) (*pb_firewall.CheckServerAccess_Response, error) {
	serverAccess := new(pb_firewall.CheckServerAccess_Response)

	// The profile is included or not
	access, err := userAccountActivity(in.GetAccountId(), in.GetUserId())
	if err != nil {
		serverAccess.Access = false
		return serverAccess, err
	}

	// Checking the country and IP addresses by Black and White sheets
	if access {
		access, err = accountActivityList(in.GetAccountId(), in.GetCountry(), in.GetClientIp())
		if err != nil {
			serverAccess.Access = false
			return serverAccess, err
		}
	}

	// Check by time
	if access {
		access, err = timeAccountActivity(in.GetAccountId())
		if err != nil {
			serverAccess.Access = false
			return serverAccess, err
		}
	}

	serverAccess.Access = access
	return serverAccess, nil
}

func countryFromIP(ip string) (*string, error) {
	db, err := geoip2.Open(internal.GetString("SECURITY_GEOIP2", "/etc/geoip2/GeoLite2-Country.mmdb"))
	if err != nil {
		return nil, errFailedToOpenFile
	}
	defer db.Close()

	record, err := db.City(net.ParseIP(ip))
	if err != nil {
		return nil, errAccessIsDenied
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

	err := service.db.Conn.QueryRow(`SELECT
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
	).Scan(&id)
	if err != nil {
		return false, errFailedToScan
	}
	if id == 0 {
		return false, errAccessIsDeniedTime // Access to this server is blocked at this time
	}

	return access, nil
}

// accessListCountry and accessListIP can return the following values:
// True - the white list is active in this server
// false - this server is active in black list
func accountActivityList(accountID, country, ip string) (bool, error) {
	if ip == "127.0.0.1" {
		return true, nil
	}

	var accessListCountry, accessListIP bool
	err := service.db.Conn.QueryRow(`SELECT
			"server_access_policy"."ip",
			"server_access_policy"."country"
		FROM
			"server_access_policy"
		WHERE
			"server_access_policy"."server_id" =$1`,
		accountID,
	).Scan(&accessListIP, &accessListCountry)
	if err != nil {
		return false, errFailedToScan
	}

	// Sample from the table with countries
	count := 0
	err = service.db.Conn.QueryRow(`SELECT
			COUNT (*)
		FROM
			"server_access_policy"
		INNER
			JOIN "server_security_country" ON "server_access_policy"."server_id"="server_security_country"."server_id"
		WHERE
			"server_access_policy"."server_id" = $1
			AND "server_security_country"."country_code" = $2`,
		accountID,
		country,
	).Scan(&count)
	if err != nil {
		return false, errFailedToScan
	}

	// Black list, the country is found in the list:
	if !accessListCountry && count > 0 {
		return false, errAccessIsDeniedCountry
	}

	// White list, the country was not found on the list
	if accessListCountry && count == 0 {
		return false, errAccessIsDeniedCountry
	}

	// We make a sample in the database with a list of IP addresses
	count = 0 // We drop the meaning
	err = service.db.Conn.QueryRow(`SELECT
      COUNT (*)
		FROM
			"server_access_policy"
			INNER JOIN "server_security_ip" ON "server_access_policy"."server_id" = "server_security_ip"."server_id"
		WHERE
			"server_access_policy"."server_id" = $1
			AND $2::inet >= "server_security_ip"."start_ip"
			AND $2::inet <= "server_security_ip"."end_ip"`,
		accountID,
		ip,
	).Scan(&count)
	if err != nil {
		return false, errFailedToScan
	}

	// Black list, IP found in the list
	if !accessListIP && count > 0 {
		return false, errAccessIsDeniedIP
	}

	// White list, IP was not found on the list
	if accessListIP && count == 0 {
		return false, errAccessIsDeniedIP
	}

	return true, nil
}

// userAccountActivity is ...
func userAccountActivity(accountID, userID string) (bool, error) {
	id := 0
	err := service.db.Conn.QueryRow(`SELECT
			"server_member"."id"
		FROM
			"server_member"
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
	).Scan(&id)
	if err != nil {
		return false, errFailedToScan
	}
	if id == 0 {
		return false, errAccessIsDeniedUser // Access is blocked
	}

	return true, nil
}

// blackListIP is checks if the IP address is in the active blacklist
// returns true if the IP address is in the active blacklist
// returns false if the IP address is not in the active blacklist
func blackListIP(ip string) (bool, error) {
	count := 0
	err := service.db.Conn.QueryRow(`SELECT
			COUNT(*)
		FROM
			"firewall_ip"
			INNER JOIN "firewall_list" ON "firewall_ip"."list_id" = "firewall_list"."id"
		WHERE
			$1::inet >= "start_ip"
			AND $1::inet <= "end_ip"
			AND "firewall_list"."active" = TRUE`,
		ip,
	).Scan(&count)
	if err != nil {
		return true, errFailedToScan
	}

	// Black list, IP found in the list
	if count > 0 {
		return true, nil
	}

	return false, nil
}
