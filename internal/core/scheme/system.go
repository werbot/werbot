package scheme

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/werbot/werbot/internal/core/firewall"
	firewallmessage "github.com/werbot/werbot/internal/core/firewall/proto/message"
	"github.com/werbot/werbot/internal/core/scheme/access"
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/strutil"
)

// SystemSchemesByAlias is ...
func (h *Handler) SystemSchemesByAlias(ctx context.Context, in *schemepb.SystemSchemesByAlias_Request) (*schemepb.SystemSchemesByAlias_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &schemepb.SystemSchemesByAlias_Response{}
	loginArray := strutil.SplitNTrimmed(in.GetAlias(), "_", 3)

	totalQuery := `
    SELECT COUNT(*)
    FROM
      "scheme"
      INNER JOIN "scheme_member" ON "scheme_member"."scheme_id" = "scheme"."id"
      INNER JOIN "project_member" ON "scheme_member"."project_member_id" = "project_member"."id"
      INNER JOIN "project" ON "project"."id" = "project_member"."project_id"
      INNER JOIN "profile" ON "profile"."id" = "project_member"."profile_id"
    WHERE
      "profile"."alias" = $1
      AND "scheme_member"."active" = TRUE
      AND "scheme"."active" = TRUE
      AND "project"."id" = "scheme"."project_id"
  `

	baseQuery := `
    SELECT DISTINCT
      "project_member"."project_id",
      "scheme"."id",
      "scheme"."scheme_type",
      CASE
        WHEN "scheme"."access"->>'key' IS NOT NULL THEN 2
        WHEN "scheme"."access"->>'agent' IS NOT NULL THEN 3
        WHEN "scheme"."access"->>'mtls' IS NOT NULL THEN 4
        WHEN "scheme"."access"->>'api' IS NOT NULL THEN 5
        ELSE 1
      END AS "auth_method",
      "project"."alias" AS "project_alias",
      "scheme"."access"->>'alias' AS "scheme_alias",
      "scheme"."title",
      "scheme"."audit",
      "scheme"."online"
    FROM
      "scheme"
      INNER JOIN "scheme_member" ON "scheme_member"."scheme_id" = "scheme"."id"
      INNER JOIN "project_member" ON "scheme_member"."project_member_id" = "project_member"."id"
      INNER JOIN "project" ON "project"."id" = "project_member"."project_id"
      INNER JOIN "profile" ON "profile"."id" = "project_member"."profile_id"
    WHERE
      "profile"."alias" = $1
      AND "scheme_member"."active" = TRUE
      AND "scheme"."active" = TRUE
      AND "project"."id" = "scheme"."project_id"
  `

	switch len(loginArray) {
	case 1:
		// No additional queries needed for case 1
	case 2:
		totalQuery = postgres.SQLGluing(totalQuery, `AND "project"."alias" = $2`)
		baseQuery = postgres.SQLGluing(baseQuery, `AND "project"."alias" = $2`)
	case 3:
		totalQuery = postgres.SQLGluing(totalQuery, `AND "project"."alias" = $2 AND "scheme"."access"->>'alias' = $3`)
		baseQuery = postgres.SQLGluing(baseQuery, `AND "project"."alias" = $2 AND "scheme"."access"->>'alias' = $3`)
	default:
		errGRPC := status.Error(codes.NotFound, trace.MsgSchemeNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// Convert loginArray to []any
	args := make([]any, len(loginArray))
	for i, v := range loginArray {
		args[i] = v
	}

	// Total count
	err := h.DB.Conn.QueryRowContext(ctx, totalQuery, args...).Scan(&response.Total)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, trace.Error(err, log, nil)
	}
	if response.Total == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgSchemeNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// Schemes data
	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		var projectAlias, schemeAlias string
		scheme := &schemepb.Scheme_Response{}
		err := rows.Scan(
			&scheme.ProjectId,
			&scheme.SchemeId,
			&scheme.SchemeType,
			&scheme.AuthMethod,
			&projectAlias,
			&schemeAlias,
			&scheme.Title,
			&scheme.Audit,
			&scheme.Online,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		scheme.Alias = fmt.Sprintf("%s_%s_%s", loginArray[0], projectAlias, schemeAlias)
		response.Schemes = append(response.Schemes, scheme)
	}

	return response, nil
}

// SystemSchemeAccess is ...
func (h *Handler) SystemSchemeAccess(ctx context.Context, in *schemepb.SystemSchemeAccess_Request) (*schemepb.SystemSchemeAccess_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	// check global firewall
	firewallHandler := &firewall.Handler{
		DB: h.DB,
	}

	ipInfo, err := firewallHandler.IPAccess(ctx, &firewallmessage.IPAccess_Request{
		ClientIp: in.GetClientIp(),
	})
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	// check access for scheme
	query := `
    WITH ctime AS (
      SELECT
        EXTRACT(DOW FROM $4::timestamp) AS "day_of_week",
        EXTRACT(HOUR FROM $4::timestamp) AS "hour_of_day"
    ),
    activity_check AS (
      SELECT
        CASE WHEN (
          CASE WHEN "day_of_week" = 0 THEN "data" -> 'sun'
              WHEN "day_of_week" = 1 THEN "data" -> 'mon'
              WHEN "day_of_week" = 2 THEN "data" -> 'tue'
              WHEN "day_of_week" = 3 THEN "data" -> 'wed'
              WHEN "day_of_week" = 4 THEN "data" -> 'thu'
              WHEN "day_of_week" = 5 THEN "data" -> 'fri'
              WHEN "day_of_week" = 6 THEN "data" -> 'sat'
            END ->> ("hour_of_day"::smallint))::integer = 1
          THEN TRUE
          ELSE FALSE
        END AS "activity_allowed"
      FROM "scheme_activity", "ctime"
      WHERE "scheme_id" = $1
    )
    SELECT
      s."project_id",
      s."scheme_type",
      s."access",
      CASE
        WHEN s."access_policy" ->> 'network' = '0' THEN NOT EXISTS (
          SELECT 1
          FROM "scheme_firewall_network" sfn
          WHERE sfn."scheme_id" = s."id" AND $2::inet <<= sfn."network"
        )
        WHEN s."access_policy" ->> 'network' = '1' THEN EXISTS (
          SELECT 1
          FROM "scheme_firewall_network" sfn
          WHERE sfn."scheme_id" = s."id" AND $2::inet <<= sfn."network"
        )
      END AS "network_allowed",
      CASE
        WHEN s."access_policy" ->> 'country' = '0' THEN NOT EXISTS (
          SELECT 1
          FROM "scheme_firewall_country" cfc
          WHERE cfc."scheme_id" = s."id" AND cfc."country_code" = $3
        )
        WHEN s."access_policy" ->> 'country' = '1' THEN EXISTS (
          SELECT 1
          FROM "scheme_firewall_country" cfc
          WHERE cfc."scheme_id" = s."id" AND cfc."country_code" = $3
        )
      END AS "country_allowed",
      ac."activity_allowed"
    FROM "scheme" s
    JOIN "activity_check" ac ON s."id" = $1
  `

	response := &schemepb.SystemSchemeAccess_Response{}

	var currentTime time.Time
	if in.GetTimestamp() != nil {
		currentTime = in.GetTimestamp().AsTime()
	} else {
		currentTime = time.Now()
	}

	var accessJSON []byte
	var countryAllowed, networkAllowed, activityAllowed pgtype.Bool
	err = h.DB.Conn.QueryRowContext(ctx, query,
		in.GetSchemeId(),
		in.GetClientIp(),
		ipInfo.GetCountryCode(),
		currentTime,
	).Scan(
		&response.ProjectId,
		&response.SchemeType,
		&accessJSON,
		&countryAllowed,
		&networkAllowed,
		&activityAllowed,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errGRPC := status.Error(codes.NotFound, trace.MsgSchemeNotFound)
			return nil, trace.Error(errGRPC, log, nil)
		}
		return nil, trace.Error(err, log, nil)
	}

	if !countryAllowed.Bool || !networkAllowed.Bool || !activityAllowed.Bool {
		var errMsg string
		switch {
		case !countryAllowed.Bool:
			errMsg = trace.MsgAccessIsDeniedCountry
		case !networkAllowed.Bool:
			errMsg = trace.MsgAccessIsDeniedIP
		case !activityAllowed.Bool:
			errMsg = trace.MsgAccessIsDeniedTime
		}
		errGRPC := status.Error(codes.PermissionDenied, errMsg)
		return nil, trace.Error(errGRPC, log, nil)
	}

	accessData, err := access.Unmarshal(accessJSON, response.GetSchemeType())
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	accessData.Ghoster()
	response.Access = accessData.AccessScheme

	return response, nil
}

// SystemHostKey is ...
func (h *Handler) SystemHostKey(ctx context.Context, in *schemepb.SystemHostKey_Request) (*schemepb.SystemHostKey_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &schemepb.SystemHostKey_Response{}
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT "host_key"
    FROM "scheme_host_key"
    WHERE "scheme_id" = $1
  `, in.GetSchemeId()).Scan(&response.Hostkey)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	return response, nil
}

// SystemUpdateHostKey is ...
func (h *Handler) SystemUpdateHostKey(ctx context.Context, in *schemepb.SystemUpdateHostKey_Request) (*schemepb.SystemUpdateHostKey_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	result, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "scheme_host_key"
    SET "host_key" = $1
    WHERE "scheme_id" = $2
  `,
		in.GetHostkey(),
		in.GetSchemeId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgSchemeNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &schemepb.SystemUpdateHostKey_Response{}, nil
}
