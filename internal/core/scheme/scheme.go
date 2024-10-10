package scheme

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/jackc/pgx/v5/pgtype"

	firewallpb "github.com/werbot/werbot/internal/core/firewall/proto/firewall"
	"github.com/werbot/werbot/internal/core/scheme/access"
	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/mathutil"
	"github.com/werbot/werbot/pkg/utils/netutil"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
)

// TODO When updating the IP address of the scheme, you need to update HostKey !!!!

// Schemes is displays a list of available schemes
func (h *Handler) Schemes(ctx context.Context, in *schemepb.Schemes_Request) (*schemepb.Schemes_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &schemepb.Schemes_Response{}
	sqlUserLimit := postgres.SQLColumnsNull(in.GetIsAdmin(), true, []string{`"scheme"."locked_at"`}) // if not admin
	schemeGroupe := mathutil.RoundToHundred(int32(in.GetSchemeType()))

	// Total count for pagination
	baseQuery := postgres.SQLGluing(`
    SELECT COUNT(*)
    FROM
      "scheme"
      INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
    WHERE
      "scheme"."project_id" = $1
      AND "project"."owner_id" = $2
      AND "scheme"."scheme_type" > $3 AND "scheme"."scheme_type" < ($3 + 99)
  `, sqlUserLimit)
	err := h.DB.Conn.QueryRowContext(ctx, baseQuery,
		in.GetProjectId(),
		in.GetOwnerId(),
		schemeGroupe,
	).Scan(&response.Total)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, trace.Error(err, log, nil)
	}
	if response.Total == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgSchemeNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// List records
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	if in.GetProjectId() != "" && in.GetOwnerId() != "" {
		baseQuery := postgres.SQLGluing(`
      SELECT DISTINCT ON ("scheme"."id")
        "scheme"."id",
        "scheme"."access"->>'address' AS address,
        "scheme"."access"->>'alias' AS alias,
        "scheme"."title",
        "scheme"."description",
        "scheme"."audit",
        "scheme"."online",
        "scheme"."active",
        "scheme"."scheme_type",
        CASE
	        WHEN "scheme"."access"->>'key' IS NOT NULL THEN 2
	        WHEN "scheme"."access"->>'agent' IS NOT NULL THEN 3
          WHEN "scheme"."access"->>'mtls' IS NOT NULL THEN 4
          WHEN "scheme"."access"->>'api' IS NOT NULL THEN 5
	        ELSE 1
	      END AS "auth_method",
        "scheme"."locked_at",
        "scheme"."archived_at",
        "scheme"."updated_at",
        "scheme"."created_at",
        (
          SELECT COUNT(*)
          FROM "scheme_member"
          WHERE "scheme_id" = "scheme"."id"
        ) as "scheme_member"
      FROM
        "scheme"
        INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
      WHERE
        "scheme"."project_id" = $1
        AND "project"."owner_id" = $2
        AND "scheme"."scheme_type" > $3 AND "scheme"."scheme_type" < ($3 + 99)
    `, sqlUserLimit, sqlFooter)

		rows, err := h.DB.Conn.QueryContext(ctx, baseQuery,
			in.GetProjectId(),
			in.GetOwnerId(),
			schemeGroupe,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}
		defer rows.Close()

		for rows.Next() {
			var address, alias sql.NullString
			var lockedAt, archivedAt, updatedAt, createdAt pgtype.Timestamp
			scheme := &schemepb.Scheme_Response{
				ProjectId: in.GetProjectId(),
			}
			err = rows.Scan(
				&scheme.SchemeId,
				&address,
				&alias,
				&scheme.Title,
				&scheme.Description,
				&scheme.Audit,
				&scheme.Online,
				&scheme.Active,
				&scheme.SchemeType,
				&scheme.AuthMethod,
				&lockedAt,
				&archivedAt,
				&updatedAt,
				&createdAt,
				&scheme.CountMembers,
			)
			if err != nil {
				return nil, trace.Error(err, log, nil)
			}

			if address.Valid {
				scheme.Address = address.String
			}

			if alias.Valid {
				scheme.Alias = alias.String
			}

			protoutils.SetPgtypeTimestamps(scheme, map[string]pgtype.Timestamp{
				"locked_at":   lockedAt,
				"archived_at": archivedAt,
				"updated_at":  updatedAt,
				"created_at":  createdAt,
			})

			if !in.GetIsAdmin() {
				ghoster.Secrets(scheme, true)
			}

			response.Schemes = append(response.Schemes, scheme)
		}
	}

	return response, nil
}

// Scheme is displays data on the scheme
func (h *Handler) Scheme(ctx context.Context, in *schemepb.Scheme_Request) (*schemepb.Scheme_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	var lockedAt, archivedAt, updatedAt, createdAt pgtype.Timestamp
	response := &schemepb.Scheme_Response{
		ProjectId: in.GetProjectId(),
	}
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "scheme"."access"->>'address' AS address,
      "scheme"."access"->>'alias' AS alias,
      "scheme"."title",
      "scheme"."description",
      "scheme"."audit",
      "scheme"."online",
      "scheme"."active",
      "scheme"."scheme_type",
      CASE
        WHEN "scheme"."access"->>'key' IS NOT NULL THEN 2
        WHEN "scheme"."access"->>'agent' IS NOT NULL THEN 3
        WHEN "scheme"."access"->>'mtls' IS NOT NULL THEN 4
        WHEN "scheme"."access"->>'api' IS NOT NULL THEN 5
        ELSE 1
      END AS "auth_method",
      "scheme"."locked_at",
      "scheme"."archived_at",
      "scheme"."updated_at",
      "scheme"."created_at"
    FROM
      "scheme"
      INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
    WHERE
      "project"."owner_id" = $1
      AND "project"."id" = $2
      AND "scheme"."id" = $3
  `,
		in.GetOwnerId(),
		in.GetProjectId(),
		in.GetSchemeId(),
	).Scan(
		&response.Address,
		&response.Alias,
		&response.Title,
		&response.Description,
		&response.Audit,
		&response.Online,
		&response.Active,
		&response.SchemeType,
		&response.AuthMethod,
		&lockedAt,
		&archivedAt,
		&updatedAt,
		&createdAt,
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	protoutils.SetPgtypeTimestamps(response, map[string]pgtype.Timestamp{
		"locked_at":   lockedAt,
		"archived_at": archivedAt,
		"updated_at":  updatedAt,
		"created_at":  createdAt,
	})

	if !in.GetIsAdmin() {
		ghoster.Secrets(response, true)
	}

	return response, nil
}

// AddScheme is ...
func (h *Handler) AddScheme(ctx context.Context, in *schemepb.AddScheme_Request) (*schemepb.AddScheme_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	// define access scheme
	accessScheme, err := access.Scheme(ctx, &access.SchemeHandler{
		DB:           h.DB,
		Redis:        h.Redis,
		AccessScheme: in.GetScheme(),
		Log:          log,
	})
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	// set default title
	schemeTitle := in.GetTitle()
	if in.GetTitle() == "" {
		schemeTitle = fmt.Sprintf(accessScheme.TitlePattern, crypto.NewPassword(6, false))
	}

	// convert scheme to json format
	var accessData []byte
	if accessData, err = protojson.Marshal(accessScheme.Access); err != nil {
		return nil, trace.Error(err, log, nil)
	}

	response := &schemepb.AddScheme_Response{}
	err = h.DB.Conn.QueryRowContext(ctx, `
    INSERT INTO "scheme" ("project_id", "title", "description", "active", "audit", "scheme_type", "access")
    SELECT $2, $3, $4, $5, $6, $7, $8
    WHERE EXISTS (
      SELECT 1
      FROM "project"
      WHERE "owner_id" = $1 AND "id" = $2
    )
    RETURNING "id"
  `,
		in.GetOwnerId(),
		in.GetProjectId(),
		schemeTitle,
		in.GetDescription(),
		in.GetActive(),
		in.GetAudit(),
		int32(accessScheme.SchemeType),
		accessData,
	).Scan(&response.SchemeId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errGRPC := status.Error(codes.Canceled, trace.MsgFailedToAdd)
			return nil, trace.Error(errGRPC, log, nil)
		}
		return nil, trace.Error(err, log, "")
	}

	return response, nil
}

// UpdateScheme is ...
func (h *Handler) UpdateScheme(ctx context.Context, in *schemepb.UpdateScheme_Request) (*schemepb.UpdateScheme_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	var column string
	var value any

	switch in.GetSetting().(type) {
	case *schemepb.UpdateScheme_Request_Title:
		column = "title"
		value = in.GetTitle()
	case *schemepb.UpdateScheme_Request_Description:
		column = "description"
		value = in.GetDescription()
	case *schemepb.UpdateScheme_Request_Audit:
		column = "audit"
		value = in.GetAudit()
	case *schemepb.UpdateScheme_Request_Active:
		column = "active"
		value = in.GetActive()
	case *schemepb.UpdateScheme_Request_Online:
		column = "online"
		value = in.GetOnline()
	case *schemepb.UpdateScheme_Request_Scheme:
		column = "access"

		// define access scheme
		accessScheme, err := access.Scheme(ctx, &access.SchemeHandler{
			DB:           h.DB,
			Redis:        h.Redis,
			AccessScheme: in.GetScheme(),
			Update: &access.OwnerScheme{
				SchemeID:  in.GetSchemeId(),
				ProjectID: in.GetProjectId(),
				OwnerID:   in.GetOwnerId(),
			},
			Log: log,
		})
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if value, err = protojson.Marshal(accessScheme.Access); err != nil {
			return nil, trace.Error(err, log, nil)
		}
	default:
		errGRPC := status.Error(codes.NotFound, trace.MsgSettingNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	query := fmt.Sprintf(`
    UPDATE "scheme"
    SET "%s" = $1
    FROM "project"
    WHERE
      "scheme"."project_id" = "project"."id"
      AND "project"."owner_id" = $2
      AND "project"."id" = $3
      AND "scheme"."id" = $4
  `, column)

	result, err := h.DB.Conn.ExecContext(ctx, query, value, in.GetOwnerId(), in.GetProjectId(), in.GetSchemeId())
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgSchemeNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &schemepb.UpdateScheme_Response{}, nil
}

// DeleteScheme is ...
func (h *Handler) DeleteScheme(ctx context.Context, in *schemepb.DeleteScheme_Request) (*schemepb.DeleteScheme_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	result, err := h.DB.Conn.ExecContext(ctx, `
    DELETE FROM "scheme"
    USING "project"
    WHERE
      "scheme"."project_id" = "project"."id"
      AND "project"."owner_id" = $1
      AND "project"."id" = $2
      AND "scheme"."id" = $3
  `,
		in.GetOwnerId(),
		in.GetProjectId(),
		in.GetSchemeId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToDelete)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgSchemeNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &schemepb.DeleteScheme_Response{}, nil
}

// SchemeAccess is displays an affordable version of connecting to the scheme
func (h *Handler) SchemeAccess(ctx context.Context, in *schemepb.SchemeAccess_Request) (*schemepb.SchemeAccess_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &schemepb.SchemeAccess_Response{}

	var schemeType schemeaccesspb.SchemeType
	var schemeByte []byte
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "scheme"."scheme_type",
      "scheme"."access"
    FROM
      "scheme"
      INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
    WHERE
      "project"."owner_id" = $1
      AND "project"."id" = $2
      AND "scheme"."id" = $3
  `,
		in.GetOwnerId(),
		in.GetProjectId(),
		in.GetSchemeId(),
	).Scan(
		&schemeType,
		&schemeByte,
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	if !in.NoGhost {
		// response.Scheme, err = access.Ghoster(schemeType, schemeByte, !in.NoGhost)
		accessUnmarshal, err := access.Unmarshal(schemeByte, schemeType)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}
		accessUnmarshal.Ghoster()
		response.Scheme = accessUnmarshal.AccessScheme
	}

	return response, nil
}

// SchemeActivity is ...
func (h *Handler) SchemeActivity(ctx context.Context, in *schemepb.SchemeActivity_Request) (*schemepb.SchemeActivity_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	var jsonb []byte
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "scheme_activity"."data"
    FROM
      "scheme_activity"
      INNER JOIN "scheme" ON "scheme_activity"."scheme_id" = "scheme"."id"
      INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
    WHERE
      "project"."owner_id" = $1
      AND "project"."id" = $2
      AND "scheme"."id" = $3
  `,
		in.GetOwnerId(),
		in.GetProjectId(),
		in.GetSchemeId(),
	).Scan(&jsonb)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	week := &schemepb.SchemeActivity_Week{}
	if err := protojson.Unmarshal(jsonb, week); err != nil {
		return nil, trace.Error(err, log, nil)
	}

	response := &schemepb.SchemeActivity_Response{}

	if in.GetTimestamp() != nil {
		timestamp := in.GetTimestamp()
		t := timestamp.AsTime()
		weekday := strings.ToLower(t.Format("Mon"))
		hour := t.Hour()

		reflectedPerson := week.ProtoReflect()
		nameField := reflectedPerson.Descriptor().Fields().ByName(protoreflect.Name(weekday))
		nameValue := reflectedPerson.Get(nameField).List().Get(hour).Int()

		response.Period = &schemepb.SchemeActivity_Response_Hour{
			Hour: nameValue != 0,
		}
	} else {
		response.Period = &schemepb.SchemeActivity_Response_Week{
			Week: week,
		}
	}

	return response, nil
}

// UpdateSchemeActivity is ...
func (h *Handler) UpdateSchemeActivity(ctx context.Context, in *schemepb.UpdateSchemeActivity_Request) (*schemepb.UpdateSchemeActivity_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	data, err := protojson.Marshal(in.GetActivity())
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	result, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "scheme_activity"
    SET "data" = $1
    FROM
      "scheme"
      INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
    WHERE
      "scheme"."id" = "scheme_activity"."scheme_id"
      AND "project"."owner_id" = $2
      AND "project"."id" = $3
      AND "scheme"."id" = $4
  `,
		data,
		in.GetOwnerId(),
		in.GetProjectId(),
		in.GetSchemeId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgSchemeNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &schemepb.UpdateSchemeActivity_Response{}, nil
}

// SchemeFirewall is scheme firewall settings for scheme_id
func (h *Handler) SchemeFirewall(ctx context.Context, in *schemepb.SchemeFirewall_Request) (*schemepb.SchemeFirewall_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &schemepb.SchemeFirewall_Response{
		Country: &schemepb.SchemeFirewall_Countries{},
		Network: &schemepb.SchemeFirewall_Networks{},
	}

	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "scheme"."access_policy"->>'country' AS country,
      (
        SELECT COUNT(*)
        FROM "scheme_firewall_country"
        WHERE "scheme_id" = "scheme"."id"
      ) as "total_country",
      "scheme"."access_policy"->>'network' AS network,
      (
        SELECT COUNT(*)
        FROM "scheme_firewall_network"
        WHERE "scheme_id" = "scheme"."id"
      ) as "total_network"
    FROM
      "scheme"
      INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
    WHERE
      "project"."owner_id" = $1
      AND "project"."id" = $2
      AND "scheme"."id" = $3
  `,
		in.GetOwnerId(),
		in.GetProjectId(),
		in.GetSchemeId(),
	).Scan(
		&response.Country.WiteList,
		&response.Country.Total,
		&response.Network.WiteList,
		&response.Network.Total,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errGRPC := status.Error(codes.NotFound, trace.MsgFirewallNotFound)
			return nil, trace.Error(errGRPC, log, nil)
		}
		return nil, trace.Error(err, log, nil)
	}

	// fetch countries
	rows, err := h.DB.Conn.QueryContext(ctx, `
    SELECT
      "scheme_firewall_country"."id",
      "scheme_firewall_country"."country_code",
      "country"."name"
    FROM
      "scheme"
      INNER JOIN "scheme_firewall_country" ON "scheme"."id" = "scheme_firewall_country"."scheme_id"
      INNER JOIN "country" ON "scheme_firewall_country"."country_code" = "country"."code"
      INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
    WHERE
      "project"."owner_id" = $1
      AND "project"."id" = $2
      AND "scheme"."id" = $3
  `,
		in.GetOwnerId(),
		in.GetProjectId(),
		in.GetSchemeId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		country := &firewallpb.Country{}
		if err := rows.Scan(&country.CountryId, &country.CountryCode, &country.CountryName); err != nil {
			return nil, trace.Error(err, log, nil)
		}
		response.Country.List = append(response.Country.List, country)
	}

	// fetch networks
	rows, err = h.DB.Conn.QueryContext(ctx, `
    SELECT
      "scheme_firewall_network"."id",
      "scheme_firewall_network"."network"
    FROM
      "scheme"
      INNER JOIN "scheme_firewall_network" ON "scheme"."id" = "scheme_firewall_network"."scheme_id"
      INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
    WHERE
      "project"."owner_id" = $1
      AND "project"."id" = $2
      AND "scheme"."id" = $3
  `,
		in.GetOwnerId(),
		in.GetProjectId(),
		in.GetSchemeId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		network := &firewallpb.Network{}
		if err := rows.Scan(&network.NetworkId, &network.Network); err != nil {
			return nil, trace.Error(err, log, nil)
		}
		response.Network.List = append(response.Network.List, network)
	}

	return response, nil
}

// AddSchemeFirewall is adding scheme firewall settings for scheme_id
func (h *Handler) AddSchemeFirewall(ctx context.Context, in *schemepb.AddSchemeFirewall_Request) (*schemepb.AddSchemeFirewall_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	var err error
	response := &schemepb.AddSchemeFirewall_Response{}

	switch in.Record.(type) {
	case *schemepb.AddSchemeFirewall_Request_CountryCode:
		var countryID sql.NullString
		err = h.DB.Conn.QueryRowContext(ctx, `
      WITH access_scheme_validate AS (
        SELECT 1
        FROM "scheme"
        INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
        WHERE
          "project"."owner_id" = $1
          AND "project"."id" = $2
          AND "scheme"."id" = $3
      )
      INSERT INTO "scheme_firewall_country" ("scheme_id", "country_code")
      SELECT  $3::uuid, $4::varchar(2)
      WHERE NOT EXISTS (
        SELECT 1
        FROM "scheme_firewall_country"
        WHERE
          "scheme_id" = $3
          AND "country_code" = $4
      )
      AND EXISTS (SELECT 1 FROM access_scheme_validate)
      RETURNING "id"
    `,
			in.GetOwnerId(),
			in.GetProjectId(),
			in.GetSchemeId(),
			in.GetCountryCode(),
		).Scan(&countryID)
		if !countryID.Valid {
			errGRPC := status.Error(codes.Canceled, trace.MsgFailedToAdd)
			return nil, trace.Error(errGRPC, log, nil)
		}
		response.Record = &schemepb.AddSchemeFirewall_Response_CountryId{
			CountryId: countryID.String,
		}

	case *schemepb.AddSchemeFirewall_Request_Network:
		var networkID sql.NullString
		network, err := netutil.IPWithMask(in.GetNetwork())
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}
		err = h.DB.Conn.QueryRowContext(ctx, `
      WITH access_scheme_validate AS (
        SELECT 1
        FROM "scheme"
        INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
        WHERE
          "project"."owner_id" = $1
          AND "project"."id" = $2
          AND "scheme"."id" = $3
      )
      INSERT INTO "scheme_firewall_network" ("scheme_id", "network")
      SELECT  $3::uuid,  $4::inet
      WHERE NOT EXISTS (
        SELECT 1
        FROM "scheme_firewall_network"
        WHERE
          "scheme_id" = $3
          AND "network" = $4
      )
      AND EXISTS (SELECT 1 FROM access_scheme_validate)
      RETURNING "id"
    `,
			in.GetOwnerId(),
			in.GetProjectId(),
			in.GetSchemeId(),
			network,
		).Scan(&networkID)
		if !networkID.Valid {
			errGRPC := status.Error(codes.Canceled, trace.MsgFailedToAdd)
			return nil, trace.Error(errGRPC, log, nil)
		}
		response.Record = &schemepb.AddSchemeFirewall_Response_NetworkId{
			NetworkId: networkID.String,
		}
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errGRPC := status.Error(codes.Canceled, trace.MsgFailedToAdd)
			return nil, trace.Error(errGRPC, log, nil)
		}
		return nil, trace.Error(err, log, "")
	}

	return response, nil
}

// UpdateSchemeFirewall is ...
func (h *Handler) UpdateSchemeFirewall(ctx context.Context, in *schemepb.UpdateSchemeFirewall_Request) (*schemepb.UpdateSchemeFirewall_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	var table string
	var statusList bool

	boolToInt32 := func(b bool) int32 {
		return map[bool]int32{true: 1, false: 0}[b]
	}

	switch in.Status.(type) {
	case *schemepb.UpdateSchemeFirewall_Request_Country:
		statusList = in.GetCountry()
		table = "{country}"
	case *schemepb.UpdateSchemeFirewall_Request_Network:
		statusList = in.GetCountry()
		table = "{network}"
	}

	result, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "scheme"
    SET "access_policy" = jsonb_set("access_policy", $4, $5)
    WHERE
      "project_id" IN (
        SELECT "id"
        FROM "project"
        WHERE
          "owner_id" = $1
          AND "id" = $2
      )
      AND "id" = $3
  `,
		in.GetOwnerId(),
		in.GetProjectId(),
		in.GetSchemeId(),
		table,
		boolToInt32(statusList),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgFirewallListNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &schemepb.UpdateSchemeFirewall_Response{}, nil
}

// DeleteSchemeFirewall is deleting scheme firewall settings for scheme_id
func (h *Handler) DeleteSchemeFirewall(ctx context.Context, in *schemepb.DeleteSchemeFirewall_Request) (*schemepb.DeleteSchemeFirewall_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	var result sql.Result
	var err error
	var msgErr string

	switch in.Record.(type) {
	case *schemepb.DeleteSchemeFirewall_Request_CountryId:
		result, err = h.DB.Conn.ExecContext(ctx, `
      DELETE FROM "scheme_firewall_country"
      WHERE "scheme_id" = $3
        AND "id" = $4
        AND EXISTS (
          SELECT 1
          FROM "project"
          JOIN "scheme" ON "scheme"."project_id" = "project"."id"
          WHERE "project"."owner_id" = $1
            AND "project"."id" = $2
            AND "scheme"."id" = $3
        )
    `,
			in.GetOwnerId(),
			in.GetProjectId(),
			in.GetSchemeId(),
			in.GetCountryId(),
		)
		msgErr = trace.MsgCountryNotFound
	case *schemepb.DeleteSchemeFirewall_Request_NetworkId:
		result, err = h.DB.Conn.ExecContext(ctx, `
      DELETE FROM "scheme_firewall_network"
      WHERE "scheme_id" = $3
        AND "id" = $4
        AND EXISTS (
          SELECT 1
          FROM "project"
          JOIN "scheme" ON "scheme"."project_id" = "project"."id"
          WHERE "project"."owner_id" = $1
            AND "project"."id" = $2
            AND "scheme"."id" = $3
        )
    `,
			in.GetOwnerId(),
			in.GetProjectId(),
			in.GetSchemeId(),
			in.GetNetworkId(),
		)
		msgErr = trace.MsgNetworkNotFound
	}

	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToDelete)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, msgErr)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &schemepb.DeleteSchemeFirewall_Response{}, nil
}
