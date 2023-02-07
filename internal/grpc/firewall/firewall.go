package firewall

import (
	"context"
	"database/sql"
	"time"

	"github.com/werbot/werbot/internal"
	firewallpb "github.com/werbot/werbot/internal/grpc/firewall/proto"
	"github.com/werbot/werbot/internal/grpc/project"
	"github.com/werbot/werbot/internal/grpc/utility"
	utilitypb "github.com/werbot/werbot/internal/grpc/utility/proto"
	"github.com/werbot/werbot/pkg/strutil"
)

// IPAccess is global service access check by IP
// old version https://git.piplos.media/werbot/werbot-server/-/blob/feature/audit-record/pkg/acl/security.go
// https://git.piplos.media/werbot/old-werbot/-/blob/master/wserver/firewall.go
func (h *Handler) IPAccess(ctx context.Context, in *firewallpb.IPAccess_Request) (*firewallpb.IPAccess_Response, error) {
	response := new(firewallpb.IPAccess_Response)

	// TODO add only if debug mode
	if in.GetClientIp() == "127.0.0.1" {
		return response, nil
	}

	if in.GetClientIp() == "" {
		return nil, errIncorrectParameters
	}

	// Verification of the country according to the global list of prohibited countries
	pbUtility := new(utility.Handler)
	responseC, _ := pbUtility.CountryByIP(ctx, &utilitypb.CountryByIP_Request{
		Ip: in.GetClientIp(),
	})
	if strutil.StringInSlice(responseC.GetName(), internal.GetSliceString("SECURITY_BAD_COUNTRY", "")) {
		return nil, errAccessIsDeniedCountry
	}

	// porch IP on the global list
	var total int32
	err := h.DB.Conn.QueryRow(`SELECT COUNT(*)
		FROM "firewall_ip"
			INNER JOIN "firewall_list" ON "firewall_ip"."list_id" = "firewall_list"."id"
		WHERE $1::inet >= "start_ip"
			AND $1::inet <= "end_ip"
			AND "firewall_list"."active" = TRUE`,
		in.GetClientIp(),
	).Scan(&total)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	// Black list, IP found in the list
	if total > 0 {
		return nil, errAccessIsDeniedIP
	}

	return response, nil
}

// ServerFirewall is server firewall settings for server_id
func (h *Handler) ServerFirewall(ctx context.Context, in *firewallpb.ServerFirewall_Request) (*firewallpb.ServerFirewall_Response, error) {
	if !project.IsOwnerProject(h.DB, in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	response := new(firewallpb.ServerFirewall_Response)
	response.Country = new(firewallpb.ServerFirewall_Countries)
	response.Network = new(firewallpb.ServerFirewall_Networks)

	// get countries
	rows, err := h.DB.Conn.Query(`SELECT "server_security_country"."id", "server_security_country"."country_code", "country"."name"
		FROM "server_security_country"
			INNER JOIN "country" ON "server_security_country"."country_code" = "country"."code"
		WHERE "server_security_country"."server_id" = $1`,
		in.GetServerId(),
	)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	for rows.Next() {
		country := new(firewallpb.Country)
		if err := rows.Scan(&country.Id, &country.CountryCode, &country.CountryName); err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			log.FromGRPC(err).Send()
			return nil, errServerError
		}
		response.Country.List = append(response.Country.List, country)
	}
	defer rows.Close()

	// get networks
	rows, err = h.DB.Conn.Query(`SELECT "id", "start_ip", "end_ip" FROM "server_security_ip" WHERE "server_id" = $1`,
		in.GetServerId(),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	for rows.Next() {
		network := new(firewallpb.Network)
		if err := rows.Scan(&network.Id, &network.StartIp, &network.EndIp); err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			log.FromGRPC(err).Send()
			return nil, errServerError
		}
		response.Network.List = append(response.Network.List, network)
	}
	defer rows.Close()

	// get status black lists
	err = h.DB.Conn.QueryRow(`SELECT "ip", "country" FROM "server_access_policy" WHERE "server_id" = $1`,
		in.GetServerId(),
	).Scan(
		&response.Network.WiteList,
		&response.Country.WiteList,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	return response, nil
}

// AddServerFirewall is adding server firewall settings for server_id
func (h *Handler) AddServerFirewall(ctx context.Context, in *firewallpb.AddServerFirewall_Request) (*firewallpb.AddServerFirewall_Response, error) {
	if !project.IsOwnerProject(h.DB, in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	response := new(firewallpb.AddServerFirewall_Response)

	switch record := in.Record.(type) {
	case *firewallpb.AddServerFirewall_Request_CountryCode:
		err := h.DB.Conn.QueryRow(`SELECT "id" FROM "server_security_country" WHERE "server_id" = $1 AND "country_code" = $2`,
			in.GetServerId(),
			record.CountryCode,
		).Scan(&response.Id)
		if err != nil && err != sql.ErrNoRows {
			log.FromGRPC(err).Send()
			return nil, errServerError
		}
		if response.Id != "" {
			return nil, errObjectAlreadyExists
		}

		err = h.DB.Conn.QueryRow(`INSERT INTO "server_security_country" ("server_id", "country_code") VALUES ($1, $2) RETURNING id`,
			in.GetServerId(),
			record.CountryCode,
		).Scan(&response.Id)
		if err != nil {
			log.FromGRPC(err).Send()
			return nil, errFailedToAdd
		}

	case *firewallpb.AddServerFirewall_Request_Ip:
		err := h.DB.Conn.QueryRow(`SELECT "id" FROM "server_security_ip" WHERE "server_id" = $1 AND "start_ip" = $2 AND "end_ip" = $3`,
			in.GetServerId(),
			record.Ip.StartIp,
			record.Ip.EndIp,
		).Scan(&response.Id)
		if err != nil && err != sql.ErrNoRows {
			log.FromGRPC(err).Send()
			return nil, errServerError
		}
		if response.Id != "" {
			return nil, errObjectAlreadyExists
		}

		err = h.DB.Conn.QueryRow(`INSERT INTO "server_security_ip" ("server_id", "start_ip", "end_ip") VALUES ($1, $2, $3) RETURNING id`,
			in.GetServerId(),
			record.Ip.StartIp,
			record.Ip.EndIp,
		).Scan(&response.Id)
		if err != nil {
			log.FromGRPC(err).Send()
			return nil, errFailedToAdd
		}

	default:
		return response, nil
	}

	return response, nil
}

// DeleteServerFirewall is deleting server firewall settings for server_id
func (h *Handler) DeleteServerFirewall(ctx context.Context, in *firewallpb.DeleteServerFirewall_Request) (*firewallpb.DeleteServerFirewall_Response, error) {
	if !project.IsOwnerProject(h.DB, in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var sql string
	response := new(firewallpb.DeleteServerFirewall_Response)

	switch in.Rule {
	case firewallpb.Rules_country:
		sql = `DELETE FROM "server_security_country" WHERE "id" = $1`
	case firewallpb.Rules_ip:
		sql = `DELETE FROM "server_security_ip" WHERE "id" = $1`
	default:
		return response, nil
	}

	data, err := h.DB.Conn.Exec(sql, in.GetRecordId())
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// UpdateServerFirewall is ...
func (h *Handler) UpdateServerFirewall(ctx context.Context, in *firewallpb.UpdateServerFirewall_Request) (*firewallpb.UpdateServerFirewall_Response, error) {
	if !project.IsOwnerProject(h.DB, in.GetProjectId(), in.GetUserId()) {
		return nil, errNotFound
	}

	var sql string
	response := new(firewallpb.UpdateServerFirewall_Response)

	switch in.Rule {
	case firewallpb.Rules_country:
		sql = `UPDATE "server_access_policy" SET "country" = $1 WHERE "server_id" = $2`
	case firewallpb.Rules_ip:
		sql = `UPDATE "server_access_policy" SET "ip" = $1 WHERE "server_id" = $2`
	default:
		return response, nil
	}

	data, err := h.DB.Conn.Exec(sql, in.GetStatus(), in.GetServerId())
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// ServerAccess is checks if the participant has access to the server according
// to the server's individual firewall settings
func (h *Handler) ServerAccess(ctx context.Context, in *firewallpb.ServerAccess_Request) (*firewallpb.ServerAccess_Response, error) {
	response := new(firewallpb.ServerAccess_Response)

	// Global service access check by IP
	if _, err := h.IPAccess(ctx, &firewallpb.IPAccess_Request{
		ClientIp: in.GetMemberIp(),
	}); err != nil {
		log.FromGRPC(err).Send()
		return nil, err
	}

	// Check by user
	if _, err := h.ServerAccessUser(ctx, &firewallpb.ServerAccessUser_Request{
		ServerId: in.GetServerId(),
		UserId:   in.GetUserId(),
	}); err != nil {
		log.FromGRPC(err).Send()
		return nil, err
	}

	// Check by ip
	if _, err := h.ServerAccessIP(ctx, &firewallpb.ServerAccessIP_Request{
		ServerId: in.GetServerId(),
		MemberIp: in.GetMemberIp(),
	}); err != nil {
		log.FromGRPC(err).Send()
		return nil, err
	}

	// Check by country
	if _, err := h.ServerAccessCountry(ctx, &firewallpb.ServerAccessCountry_Request{
		ServerId: in.GetServerId(),
		MemberIp: in.GetMemberIp(),
	}); err != nil {
		log.FromGRPC(err).Send()
		return nil, err
	}

	// Check by time
	if _, err := h.ServerAccessTime(ctx, &firewallpb.ServerAccessTime_Request{
		ServerId: in.GetServerId(),
	}); err != nil {
		log.FromGRPC(err).Send()
		return nil, err
	}

	return response, nil
}

// ServerAccessUser is ...
func (h *Handler) ServerAccessUser(ctx context.Context, in *firewallpb.ServerAccessUser_Request) (*firewallpb.ServerAccessUser_Response, error) {
	memberID := ""
	response := new(firewallpb.ServerAccessUser_Response)

	err := h.DB.Conn.QueryRow(`SELECT "server_member"."id"
		FROM "server_member"
			INNER JOIN "project_member" ON "server_member"."member_id" = "project_member"."id"
			INNER JOIN "server" ON "server_member"."server_id" = "server"."id"
		WHERE "project_member"."user_id" = $2
			AND "project_member"."active" = TRUE
			AND "server_member"."server_id" = $1
			AND "server_member"."active" = TRUE
			AND "server"."active" = TRUE`,
		in.GetServerId(),
		in.GetUserId(),
	).Scan(&memberID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		log.FromGRPC(err).Send()
		return nil, errServerError
	}
	if memberID == "" {
		return nil, errAccessIsDeniedUser
	}

	return response, nil
}

// ServerAccessTime is checks if it is possible to connect to the server now
func (h *Handler) ServerAccessTime(ctx context.Context, in *firewallpb.ServerAccessTime_Request) (*firewallpb.ServerAccessTime_Response, error) {
	id := 0
	weekDays := [...]int32{7, 1, 2, 3, 4, 5, 6}
	response := new(firewallpb.ServerAccessTime_Response)

	err := h.DB.Conn.QueryRow(`SELECT "id" FROM "server_activity"
    WHERE "server_id" = $1
      AND "dow" = $2
      AND "time_from" < $3
      AND "time_to" > $3`,
		in.GetServerId(),
		weekDays[time.Now().Weekday()],
		time.Now().Local().Format("15:04:05"),
	).Scan(&id)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errServerError
	}
	if id == 0 {
		return nil, errAccessIsDeniedTime
	}

	return response, nil
}

// ServerAccessIP is ...
func (h *Handler) ServerAccessIP(ctx context.Context, in *firewallpb.ServerAccessIP_Request) (*firewallpb.ServerAccessIP_Response, error) {
	total := 0
	response := new(firewallpb.ServerAccessIP_Response)

	// TODO add only if debug mode
	if in.GetMemberIp() == "127.0.0.1" {
		return response, nil
	}

	var accessListIP bool
	err := h.DB.Conn.QueryRow(`SELECT "ip" FROM "server_access_policy" WHERE "server_id" =$1`,
		in.GetServerId(),
	).Scan(
		&accessListIP,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	// We make a sample in the database with a list of IP addresses
	err = h.DB.Conn.QueryRow(`SELECT COUNT (*)
		FROM "server_access_policy"
			INNER JOIN "server_security_ip" ON "server_access_policy"."server_id" = "server_security_ip"."server_id"
		WHERE "server_access_policy"."server_id" = $1
			AND $2::inet >= "server_security_ip"."start_ip"
			AND $2::inet <= "server_security_ip"."end_ip"`,
		in.GetServerId(),
		in.GetMemberIp(),
	).Scan(&total)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	// Black list, IP found in the list
	if !accessListIP && total > 0 {
		return nil, errAccessIsDeniedIP
	}

	// White list, IP was not found on the list
	if accessListIP && total == 0 {
		return nil, errAccessIsDeniedIP
	}

	return response, nil
}

// ServerAccessCountry is ...
func (h *Handler) ServerAccessCountry(ctx context.Context, in *firewallpb.ServerAccessCountry_Request) (*firewallpb.ServerAccessCountry_Response, error) {
	total := 0
	response := new(firewallpb.ServerAccessCountry_Response)

	// TODO add only if debug mode
	if in.GetMemberIp() == "127.0.0.1" {
		return response, nil
	}

	pbUtility := new(utility.Handler)
	responseC, _ := pbUtility.CountryByIP(ctx, &utilitypb.CountryByIP_Request{
		Ip: in.GetMemberIp(),
	})
	country := responseC.Code

	var accessListCountry bool
	err := h.DB.Conn.QueryRow(`SELECT "country" FROM "server_access_policy" WHERE "server_id" =$1`,
		in.GetServerId(),
	).Scan(
		&accessListCountry,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	// Sample from the table with countries
	err = h.DB.Conn.QueryRow(`SELECT COUNT (*)
		FROM "server_access_policy"
		  INNER JOIN "server_security_country" ON "server_access_policy"."server_id"="server_security_country"."server_id"
		WHERE "server_access_policy"."server_id" = $1
			AND "server_security_country"."country_code" = $2`,
		in.GetServerId(),
		country,
	).Scan(&total)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	// Black list, the country is found in the list:
	if !accessListCountry && total > 0 {
		return nil, errAccessIsDeniedCountry
	}

	// White list, the country was not found on the list
	if accessListCountry && total == 0 {
		return nil, errAccessIsDeniedCountry
	}

	return response, nil
}
