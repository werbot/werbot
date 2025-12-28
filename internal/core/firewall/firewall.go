package firewall

import (
	"bufio"
	"context"
	"net/http"
	"regexp"

	firewallmessage "github.com/werbot/werbot/internal/core/firewall/proto/message"
	"github.com/werbot/werbot/internal/core/system"
	systemmessage "github.com/werbot/werbot/internal/core/system/proto/message"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/utils/netutil"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IPAccess is global service access check by IP
func (h *Handler) IPAccess(ctx context.Context, in *firewallmessage.IPAccess_Request) (*firewallmessage.IPAccess_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	utilityHandler := &system.Handler{}
	countryByIP, _ := utilityHandler.CountryByIP(ctx, &systemmessage.CountryByIP_Request{
		Ip: in.GetClientIp(),
	})

	// search country on the global list
	var total int
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM
      "firewall_country"
      INNER JOIN "firewall_list" ON "firewall_country"."firewall_list_id" = "firewall_list"."id"
    WHERE
      "firewall_country"."country_code" = $1
      AND "firewall_list"."active" = TRUE
  `, countryByIP.GetCode()).Scan(&total)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	if total > 0 {
		errGRPC := status.Error(codes.PermissionDenied, trace.MsgAccessIsDeniedCountry)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// search IP on the global list
	err = h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM
      "firewall_network"
      INNER JOIN "firewall_list" ON "firewall_network"."firewall_list_id" = "firewall_list"."id"
    WHERE
      $1::inet <<= "network"
      AND "firewall_list"."active" = TRUE
  `, in.GetClientIp()).Scan(&total)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	if total > 0 {
		errGRPC := status.Error(codes.PermissionDenied, trace.MsgAccessIsDeniedIP)
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &firewallmessage.IPAccess_Response{
		CountryName: countryByIP.GetName(),
		CountryCode: countryByIP.GetCode(),
	}

	return response, nil
}

// UpdateFirewallListData is ...
func (h *Handler) UpdateFirewallListData(ctx context.Context, _ *firewallmessage.UpdateFirewallListData_Request) (*firewallmessage.UpdateFirewallListData_Response, error) {
	rows, err := h.DB.Conn.QueryContext(ctx, `
    SELECT "id", "name", "path"
    FROM "firewall_list"
    WHERE "active" = TRUE AND "path" > 0
  `)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	re := regexp.MustCompile(`^(?:\d{1,3}\.){3}\d{1,3}(?:/\d{1,2})?`)

	for rows.Next() {
		var id, name, path string
		if err := rows.Scan(&id, &name, &path); err != nil {
			return nil, trace.Error(err, log, nil)
		}

		resp, err := http.Get("https://raw.githubusercontent.com/firehol/blocklist-ipsets/refs/heads/master/" + name)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, trace.Error(err, log, nil)
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			matches := re.FindAllString(line, -1)
			if len(matches) > 0 && !netutil.IsReservedIP(matches[0]) {
				network, err := netutil.IPWithMask(matches[0])
				if err != nil {
					log.Error(err).Msg("Error parsing IP range")
				}

				_, err = h.DB.Conn.ExecContext(ctx, `
          WITH existing_network AS (
            SELECT 1
            FROM "firewall_network"
            WHERE "firewall_list_id" = $1
              AND $2::inet <<= "network"
          )
          INSERT INTO "firewall_network" ("firewall_list_id", "network")
          SELECT $1::uuid, $2::inet
          WHERE NOT EXISTS (SELECT * FROM existing_network)
        `, id, network)
				if err != nil {
					log.Error(err).Msg("Error parsing IP range")
				}
			}
		}

		if err := scanner.Err(); err != nil {
			return nil, trace.Error(err, log, nil)
		}
	}

	return &firewallmessage.UpdateFirewallListData_Response{}, nil
}
