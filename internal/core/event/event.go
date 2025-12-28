package event

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	eventenum "github.com/werbot/werbot/internal/core/event/proto/enum"
	eventmessage "github.com/werbot/werbot/internal/core/event/proto/message"
	eventpb "github.com/werbot/werbot/internal/core/event/proto/rpc"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
)

// Events is ...
func (h *Handler) Events(ctx context.Context, in *eventpb.Events_Request) (*eventpb.Events_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &eventpb.Events_Response{}

	var sqlQuery, sqlQueryTotal string
	var args []any
	var eventType eventenum.Section

	switch in.GetRelatedId().(type) {
	case *eventpb.Events_Request_ProfileId:
		eventType = eventenum.Section_profile
		sqlQueryTotal = `
      SELECT COUNT("event"."id")
      FROM
        "event"
        INNER JOIN "profile" ON "event"."related_id" = "profile"."id"
      WHERE
        "profile"."id" = $1
        AND "event"."related_id" = $2
        AND "event"."prime_section" = 1
    `
		sqlQuery = `
      SELECT
        "event"."id",
        "profile"."id",
        "event"."section",
        "event"."type",
        "event"."session"->>'id',
        "event"."session"->>'ip',
        "event"."created_at"
      FROM
        "event"
        INNER JOIN "profile" ON "event"."related_id" = "profile"."id"
      WHERE
        "profile"."id" = $1
        AND "event"."related_id" = $2
        AND "event"."prime_section" = 1
    `
		args = append(args, in.GetOwnerId(), in.GetProfileId())

	case *eventpb.Events_Request_ProjectId:
		eventType = eventenum.Section_project
		sqlQueryTotal = `
      SELECT COUNT("event"."id")
      FROM
        "event"
        INNER JOIN "project" ON "event"."related_id" = "project"."id"
      WHERE
        "project"."owner_id" = $1
        AND "event"."related_id" = $2
        AND "event"."prime_section" = 2
    `
		sqlQuery = `
      SELECT
        "event"."id",
        "project"."owner_id",
        "event"."section",
        "event"."type",
        "event"."session"->>'id',
        "event"."session"->>'ip',
        "event"."created_at"
      FROM
        "event"
        INNER JOIN "project" ON "event"."related_id" = "project"."id"
      WHERE
        "project"."owner_id" = $1
        AND "event"."related_id" = $2
        AND "event"."prime_section" = 2
    `
		args = append(args, in.GetOwnerId(), in.GetProjectId())

	case *eventpb.Events_Request_SchemeId:
		eventType = eventenum.Section_scheme
		sqlQueryTotal = `
      SELECT COUNT("event"."id")
      FROM
        "event"
        INNER JOIN "scheme" ON "event"."related_id" = "scheme"."id"
        INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
      WHERE
        "project"."owner_id" = $1
        AND "event"."related_id" = $2
        AND "event"."prime_section" = 3
    `
		sqlQuery = `
      SELECT
        "event"."id",
        "project"."owner_id",
        "event"."section",
        "event"."type",
        "event"."session"->>'id',
        "event"."session"->>'ip',
        "event"."created_at"
      FROM
        "event"
        INNER JOIN "scheme" ON "event"."related_id" = "scheme"."id"
        INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
      WHERE
        "project"."owner_id" = $1
        AND "event"."related_id" = $2
        AND "event"."prime_section" = 3
    `
		args = append(args, in.GetOwnerId(), in.GetSchemeId())
	}

	// Total count for pagination
	err := h.DB.Conn.QueryRowContext(ctx, sqlQueryTotal, args...).Scan(&response.Total)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, trace.Error(err, log, nil)
	}
	if response.Total == 0 {
		var notFound string
		switch eventType {
		case eventenum.Section_profile:
			notFound = trace.MsgProfileNotFound
		case eventenum.Section_project:
			notFound = trace.MsgProjectNotFound
		case eventenum.Section_scheme:
			notFound = trace.MsgSchemeNotFound
		}

		errGRPC := status.Error(codes.NotFound, notFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// List events
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	baseQuery := postgres.SQLGluing(sqlQuery, sqlFooter)
	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		var relatedID string
		var createdAt pgtype.Timestamp
		event := &eventpb.Event_Response{
			Session: &eventmessage.Session{},
		}

		err = rows.Scan(
			&relatedID,
			&event.OwnerId,
			&event.Section,
			&event.Type,
			&event.Session.Id,
			&event.Session.Ip,
			&createdAt,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		switch eventType {
		case eventenum.Section_profile:
			event.RelatedId = &eventpb.Event_Response_ProfileId{ProfileId: relatedID}
		case eventenum.Section_project:
			event.RelatedId = &eventpb.Event_Response_ProjectId{ProjectId: relatedID}
		case eventenum.Section_scheme:
			event.RelatedId = &eventpb.Event_Response_SchemeId{SchemeId: relatedID}
		}

		protoutils.SetPgtypeTimestamps(response, map[string]pgtype.Timestamp{
			"created_at": createdAt,
		})

		// Clearing certain fields if the user is not an administrator
		if !in.IsAdmin {
			ghoster.Secrets(event, true)
		}

		response.Events = append(response.Events, event)
	}

	return response, nil
}

// Event is ...
func (h *Handler) Event(ctx context.Context, in *eventpb.Event_Request) (*eventpb.Event_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	var sqlQuery string
	var args []any
	var eventType string

	switch in.GetEventId().(type) {
	case *eventpb.Event_Request_ProfileId:
		eventType = "profile"
		sqlQuery = `
      SELECT
        "event"."related_id",
        "event"."section",
        "event"."type",
        "event"."session"->>'id',
        "event"."session"->>'user_agent',
        "event"."session"->>'ip',
        "event"."data",
        "event"."created_at"
      FROM
        "event"
        INNER JOIN "profile" ON "event"."related_id" = "profile"."id"
      WHERE
        "profile"."id" = $1
        AND "event"."id" = $2
        AND "event"."prime_section" = 1
    `
		args = append(args, in.GetOwnerId(), in.GetProfileId())

	case *eventpb.Event_Request_ProjectId:
		eventType = "project"
		sqlQuery = `
      SELECT
        "event"."related_id",
        "event"."section",
        "event"."type",
        "event"."session"->>'id',
        "event"."session"->>'user_agent',
        "event"."session"->>'ip',
        "event"."data",
        "event"."created_at"
      FROM
        "event"
        INNER JOIN "project" ON "event"."related_id" = "project"."id"
      WHERE
        "project"."owner_id" = $1
        AND "event"."id" = $2
        AND "event"."prime_section" = 2
    `
		args = append(args, in.GetOwnerId(), in.GetProjectId())

	case *eventpb.Event_Request_SchemeId:
		eventType = "scheme"
		sqlQuery = `
      SELECT
        "event"."related_id",
        "event"."section",
        "event"."type",
        "event"."session"->>'id',
        "event"."session"->>'user_agent',
        "event"."session"->>'ip',
        "event"."data",
        "event"."created_at"
      FROM
        "event"
        INNER JOIN "scheme" ON "event"."related_id" = "scheme"."id"
        INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
      WHERE
        "project"."owner_id" = $1
        AND "event"."id" = $2
        AND "event"."prime_section" = 3
    `
		args = append(args, in.GetOwnerId(), in.GetSchemeId())
	}

	var createdAt pgtype.Timestamp
	var related_id string
	response := &eventpb.Event_Response{
		Session: &eventmessage.Session{},
	}

	err := h.DB.Conn.QueryRowContext(ctx, sqlQuery, args...).Scan(
		&related_id,
		&response.Section,
		&response.Type,
		&response.Session.Id,
		&response.Session.UserAgent,
		&response.Session.Ip,
		&response.MetaData,
		&createdAt,
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	switch eventType {
	case "profile":
		response.RelatedId = &eventpb.Event_Response_ProfileId{ProfileId: related_id}
	case "project":
		response.RelatedId = &eventpb.Event_Response_ProjectId{ProjectId: related_id}
	case "scheme":
		response.RelatedId = &eventpb.Event_Response_SchemeId{SchemeId: related_id}
	}

	protoutils.SetPgtypeTimestamps(response, map[string]pgtype.Timestamp{
		"created_at": createdAt,
	})

	// Clearing certain fields if the user is not an administrator
	if !in.IsAdmin {
		ghoster.Secrets(response, true)
	}

	return response, nil
}

// AddEvent is ...
func (h *Handler) AddEvent(ctx context.Context, in *eventpb.AddEvent_Request) (*eventpb.AddEvent_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	var sqlCheck, ownerID, relatedID string
	var primeSection, section int32

	switch in.Section.(type) {
	case *eventpb.AddEvent_Request_Profile:
		sqlCheck = `
      WITH access_check AS (
        SELECT 1
        FROM "profile"
        WHERE "id" = $1
      )
    `
		ownerID = in.GetOwnerId()
		primeSection = int32(eventenum.Section_profile)
		section = int32(in.GetProfile().GetSection())
		relatedID = in.GetProfile().GetId()

	case *eventpb.AddEvent_Request_Project:
		sqlCheck = `
      WITH access_check AS (
        SELECT 1
        FROM "project"
        WHERE
          "id" = $2
          AND "owner_id" = $1
      )
    `
		ownerID = in.GetOwnerId()
		primeSection = int32(eventenum.Section_project)
		section = int32(in.GetProject().Section)
		relatedID = in.GetProject().GetId()

	case *eventpb.AddEvent_Request_Scheme:
		sqlCheck = `
      WITH access_check AS (
        SELECT 1
        FROM
          "scheme"
          INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
        WHERE
          "project"."owner_id" = $1
          AND "scheme"."id" = $2
      )
    `
		ownerID = in.GetOwnerId()
		primeSection = int32(eventenum.Section_scheme)
		section = int32(in.GetScheme().Section)
		relatedID = in.GetScheme().GetId()
	}

	session, err := protojson.Marshal(in.GetSession())
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	metaData := string(in.GetMetaData())
	if metaData == "" {
		metaData = "{}"
	}

	response := &eventpb.AddEvent_Response{}

	baseQuery := postgres.SQLGluing(sqlCheck, `
    INSERT INTO "event" (
      "related_id",
      "prime_section",
      "section",
      "type",
      "session",
      "data"
    )
    SELECT $2, $3, $4, $5, $6, $7
    FROM "access_check"
    RETURNING "id"
  `)

	err = h.DB.Conn.QueryRowContext(ctx, baseQuery,
		ownerID,
		relatedID,
		primeSection,
		section,
		in.GetType(),
		session,
		metaData,
	).Scan(&response.RecordId)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
	}

	return response, nil
}
