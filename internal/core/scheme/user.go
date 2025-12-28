package scheme

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/mathutil"
	"github.com/werbot/werbot/pkg/utils/protoutils"
)

// ProfileSchemes is ...
func (h *Handler) ProfileSchemes(ctx context.Context, in *schemepb.ProfileSchemes_Request) (*schemepb.ProfileSchemes_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &schemepb.ProfileSchemes_Response{
		Total: make(map[int32]int32),
	}

	// Total projects count
	var maxSchemeKey int32
	for key := range schemeaccesspb.SchemeType_name {
		if key > maxSchemeKey {
			maxSchemeKey = key
		}
	}
	maxSchemeKey -= 100

	var total int32
	projects, err := h.DB.Conn.QueryContext(ctx, `
    WITH RECURSIVE "groups" AS (
      SELECT 100 AS "group"
      UNION ALL
      SELECT "group" + 100 FROM "groups" WHERE "group" < $1
    )
    SELECT "groups"."group", COALESCE(t."count", 0) AS "count"
    FROM
      "groups"
      LEFT JOIN ( SELECT (("scheme"."scheme_type" / 100) * 100) AS "group", COUNT(*) AS "count"
        FROM
          "project_member"
          INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
          INNER JOIN "scheme_member" ON "project_member"."id" = "scheme_member"."project_member_id"
          INNER JOIN "scheme" ON "scheme_member"."scheme_id" = "scheme"."id"
        WHERE
          "project_member"."profile_id" = $2
          AND "scheme"."project_id" = "project"."id"
        GROUP BY "group") t ON "groups"."group" = t."group"
    ORDER BY "groups"."group"`,
		maxSchemeKey,
		in.GetProfileId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer projects.Close()

	for projects.Next() {
		var group, count int32
		err = projects.Scan(
			&group,
			&count,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		total = total + count
		response.Total[group] = count
	}
	if total == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgSchemeNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// List records
	var schemeType string
	if in.GetSchemeType() > 0 {
		schemeGroupe := mathutil.RoundToHundred(int32(in.GetSchemeType()))
		schemeType = fmt.Sprintf(`AND "scheme"."scheme_type" > %v AND "scheme"."scheme_type" < (%v + 99)`, schemeGroupe, schemeGroupe)
	}
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	baseQuery := postgres.SQLGluing(`
    SELECT
      "project"."id",
      "scheme"."id",
      "scheme"."access"->>'alias' AS "alias",
      "scheme"."title",
      "scheme"."active",
      "scheme"."scheme_type",
        CASE
        WHEN "scheme"."access"->>'key' IS NOT NULL THEN 2
        WHEN "scheme"."access"->>'agent' IS NOT NULL THEN 3
        WHEN "scheme"."access"->>'mtls' IS NOT NULL THEN 4
        WHEN "scheme"."access"->>'api' IS NOT NULL THEN 5
        ELSE 1
      END AS "auth_method"
    FROM
      "project_member"
      INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
      INNER JOIN "scheme_member" ON "project_member"."id" = "scheme_member"."project_member_id"
      INNER JOIN "scheme" ON "scheme_member"."scheme_id" = "scheme"."id"
    WHERE
      "project_member"."profile_id" = $1
      AND "scheme"."project_id" = "project"."id"
  `, schemeType, sqlFooter)

	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery, in.GetProfileId())
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		// var address pgtype.Text
		scheme := &schemepb.Scheme_Response{}
		err = rows.Scan(
			&scheme.ProjectId,
			&scheme.SchemeId,
			&scheme.Alias,
			&scheme.Title,
			&scheme.Active,
			&scheme.AuthMethod,
			&scheme.SchemeType,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		// scheme.Address = address.String
		response.Schemes = append(response.Schemes, scheme)
	}

	return response, nil
}
