package event

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	eventpb "github.com/werbot/werbot/internal/core/event/proto/event"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
)

// Events is ...
func (h *Handler) Events(ctx context.Context, in *eventpb.Events_Request) (*eventpb.Events_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	var sqlQuery, sqlQueryTotal string
	var args []any
	response := &eventpb.Events_Response{}
	var eventType eventpb.EventSection

	switch in.GetId().(type) {
	case *eventpb.Events_Request_UserId:
		eventType = eventpb.EventSection_profile
		sqlQueryTotal = `
      SELECT COUNT("id")
      FROM "event_profile"
      WHERE "profile_id" = $1 AND "profile_id" = $2
    `
		sqlQuery = `
      SELECT
        "id",
        "profile_id",
        "session_id",
        "ip",
        "event",
        "section",
        "created_at"
      FROM "event_profile"
      WHERE "profile_id" = $1 AND "profile_id" = $2
    `
		args = append(args, in.GetProfileId(), in.GetUserId())

	case *eventpb.Events_Request_ProjectId:
		eventType = eventpb.EventSection_project
		sqlQueryTotal = `
      SELECT COUNT("event_project"."id")
      FROM
        "event_project"
        INNER JOIN "project" ON "event_project"."project_id" = "project"."id"
      WHERE
        "event_project"."project_id" = $1
        AND "project"."owner_id" = $2
    `
		sqlQuery = `
      SELECT
        "event_project"."id",
        "event_project"."profile_id",
        "event_project"."session_id",
        "event_project"."ip",
        "event_project"."event",
        "event_project"."section",
        "event_project"."created_at"
      FROM
        "event_project"
        INNER JOIN "project" ON "event_project"."project_id" = "project"."id"
      WHERE
        "event_project"."project_id" = $1
        AND "project"."owner_id" = $2
    `
		args = append(args, in.GetProjectId(), in.GetUserId())

	case *eventpb.Events_Request_SchemeId:
		eventType = eventpb.EventSection_scheme
		sqlQueryTotal = `
      SELECT COUNT("event_scheme"."id")
      FROM
        "event_scheme"
        INNER JOIN "scheme" ON "event_scheme"."scheme_id" = "scheme"."id"
        INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
      WHERE
        "event_scheme"."scheme_id" = $1
        AND "project"."owner_id" = $2
    `
		sqlQuery = `
      SELECT
        "event_scheme"."id",
        "event_scheme"."profile_id",
        "event_scheme"."session_id",
        "event_scheme"."ip",
        "event_scheme"."event",
        "event_scheme"."section",
        "event_scheme"."created_at"
      FROM
        "event_scheme"
        INNER JOIN "scheme" ON "event_scheme"."scheme_id" = "scheme"."id"
        INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
      WHERE
        "event_scheme"."scheme_id" = $1
        AND "project"."owner_id" = $2
    `
		args = append(args, in.GetSchemeId(), in.GetUserId())
	}

	// Total count for pagination
	err := h.DB.Conn.QueryRowContext(ctx, sqlQueryTotal, args...).Scan(&response.Total)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, trace.Error(err, log, nil)
	}
	if response.Total == 0 {
		var notFound string
		switch eventType {
		case eventpb.EventSection_profile:
			notFound = trace.MsgProfileNotFound
		case eventpb.EventSection_project:
			notFound = trace.MsgProjectNotFound
		case eventpb.EventSection_scheme:
			notFound = trace.MsgSchemeNotFound
		}

		errGRPC := status.Error(codes.NotFound, notFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// List records
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	baseQuery := postgres.SQLGluing(sqlQuery, sqlFooter)
	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var createdAt pgtype.Timestamp
		record := &eventpb.Event_Response{}

		err = rows.Scan(
			&id,
			&record.ProfileId,
			&record.SessionId,
			&record.Ip,
			&record.Event,
			&record.Section,
			&createdAt,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		switch eventType {
		case eventpb.EventSection_profile:
			record.Id = &eventpb.Event_Response_UserId{
				UserId: id,
			}
		case eventpb.EventSection_project:
			record.Id = &eventpb.Event_Response_ProjectId{
				ProjectId: id,
			}
		case eventpb.EventSection_scheme:
			record.Id = &eventpb.Event_Response_SchemeId{
				SchemeId: id,
			}
		}

		protoutils.SetPgtypeTimestamps(response, map[string]pgtype.Timestamp{
			"created_at": createdAt,
		})

		// Clearing certain fields if the user is not an administrator
		if !in.IsAdmin {
			ghoster.Secrets(record, true)
		}

		response.Records = append(response.Records, record)
	}

	return response, nil
}

// Event is ...
func (h *Handler) Event(ctx context.Context, in *eventpb.Event_Request) (*eventpb.Event_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	var sqlQuery string
	var args []any
	var eventType string

	switch in.GetId().(type) {
	case *eventpb.Event_Request_UserId:
		eventType = "profile"
		sqlQuery = `
      SELECT
        "id",
        "profile_id",
        "session_id",
        "profile_agent",
        "ip",
        "event",
        "section",
        "data",
        "created_at"
      FROM "event_profile"
      WHERE
        "id" = $1
        AND "profile_id" = $2
    `
		args = append(args, in.GetProfileId(), in.GetUserId())

	case *eventpb.Event_Request_ProjectId:
		eventType = "project"
		sqlQuery = `
      SELECT
        "event_project"."project_id",
        "event_project"."profile_id",
        "event_project"."session_id",
        "event_project"."profile_agent",
        "event_project"."ip",
        "event_project"."event",
        "event_project"."section",
        "event_project"."data",
        "event_project"."created_at"
      FROM
        "event_project"
        INNER JOIN "project" ON "event_project"."project_id" = "project"."id"
      WHERE
        "event_project"."id" = $1
        AND "project"."owner_id" = $2
    `
		args = append(args, in.GetProjectId(), in.GetUserId())

	case *eventpb.Event_Request_SchemeId:
		eventType = "scheme"
		sqlQuery = `
      SELECT
        "event_scheme"."scheme_id",
        "event_scheme"."profile_id",
        "event_scheme"."session_id",
        "event_scheme"."profile_agent",
        "event_scheme"."ip",
        "event_scheme"."event",
        "event_scheme"."section",
        "event_scheme"."data",
        "event_scheme"."created_at"
      FROM
        "event_scheme"
        INNER JOIN "scheme" ON "event_scheme"."scheme_id" = "scheme"."id"
        INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
      WHERE
        "event_scheme"."id" = $1
        AND "project"."owner_id" = $2
    `
		args = append(args, in.GetSchemeId(), in.GetUserId())
	}

	var createdAt pgtype.Timestamp
	var id string
	response := &eventpb.Event_Response{}

	err := h.DB.Conn.QueryRowContext(ctx, sqlQuery, args...).Scan(
		&id,
		&response.ProfileId,
		&response.SessionId,
		&response.UserAgent,
		&response.Ip,
		&response.Event,
		&response.Section,
		&response.MetaData,
		&createdAt,
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	switch eventType {
	case "profile":
		response.Id = &eventpb.Event_Response_UserId{
			UserId: id,
		}
	case "project":
		response.Id = &eventpb.Event_Response_ProjectId{
			ProjectId: id,
		}
	case "scheme":
		response.Id = &eventpb.Event_Response_SchemeId{
			SchemeId: id,
		}
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
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &eventpb.AddEvent_Response{}

	var sqlQuery, id, profileID string
	var section int32

	switch in.Section.(type) {
	case *eventpb.AddEvent_Request_Profile:
		sqlQuery = `
      WITH profile_exists AS (
        SELECT 1
        FROM "profile"
        WHERE "id" = $2
      )
      INSERT INTO "event_profile" (
        "profile_id",
        "profile_id",
        "session_id",
        "profile_agent",
        "ip",
        "event",
        "section",
        "data"
      )
      SELECT $1, $2, $3, $4, $5, $6, $7, $8
      FROM "profile_exists"
      RETURNING "id"
    `
		section = int32(in.GetProfile().GetSection())
		id = in.GetProfile().Id
		profileID = id

	case *eventpb.AddEvent_Request_Project:
		sqlQuery = `
      WITH profile_exists AS (
        SELECT 1
        FROM "profile"
        WHERE "id" = $2
      ),
      project_check AS (
        SELECT 1
        FROM "project"
        WHERE id = $1
      )
      INSERT INTO "event_project" (
        "project_id",
        "profile_id",
        "session_id",
        "profile_agent",
        "ip",
        "event",
        "section",
        "data"
      )
      SELECT $1, $2, $3, $4, $5, $6, $7, $8
      FROM "profile_exists", "project_check"
      RETURNING "id"
    `
		section = int32(in.GetProject().Section)
		id = in.GetProject().Id
		profileID = in.GetProfileId()

	case *eventpb.AddEvent_Request_Scheme:
		sqlQuery = `
      WITH profile_exists AS (
        SELECT 1
        FROM "profile"
        WHERE "id" = $2
      ),
      scheme_check AS (
        SELECT 1
        FROM "scheme"
        WHERE id = $1
      )
      INSERT INTO
        "event_scheme" (
        "scheme_id",
        "profile_id",
        "session_id",
        "profile_agent",
        "ip",
        "event",
        "section",
        "data"
      )
      SELECT $1, $2, $3, $4, $5, $6, $7, $8
      FROM "profile_exists", "scheme_check"
      RETURNING "id"
    `
		section = int32(in.GetScheme().Section)
		id = in.GetScheme().Id
		profileID = in.GetProfileId()
	}

	tx, err := h.DB.Conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCreateError)
	}
	defer tx.Rollback()

	json := string(in.GetMetaData())
	if json == "" {
		json = "{}"
	}

	err = tx.QueryRowContext(ctx, sqlQuery,
		id,
		profileID,
		in.GetSessionId(),
		in.GetUserAgent(),
		in.GetIp(),
		in.GetEvent(),
		section,
		json,
	).Scan(&response.RecordId)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
	}

	if err := tx.Commit(); err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCommitError)
	}

	return response, nil
}
