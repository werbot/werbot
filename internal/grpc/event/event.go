package event

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventpb "github.com/werbot/werbot/internal/grpc/event/proto"
	"github.com/werbot/werbot/internal/trace"
)

// Events is ...
func (h *Handler) Events(ctx context.Context, in *eventpb.Events_Request) (*eventpb.Events_Response, error) {
	var sqlQuery, sqlQueryTotal string
	var args []any
	response := &eventpb.Events_Response{}

	switch in.GetId().(type) {
	// setting for profile events
	// use only profile_id
	case *eventpb.Events_Request_ProfileId:
		sqlQueryTotal = `
      SELECT
        COUNT("id")
      FROM
        "event_profile"
      WHERE
        "profile_id" = $1
    `
		sqlQuery = `
      SELECT
        "id",
        "user_id",
        "ip",
        "event",
        "section",
        "created_at"
      FROM
        "event_profile"
      WHERE
        "profile_id" = $1
    `
		args = append(args, in.GetProfileId())

	case *eventpb.Events_Request_ProjectId:
		// setting for project events
		// use profile_id and user_id
		if in.GetUserId() == "" {
			return nil, status.Error(codes.InvalidArgument, trace.MsgInvalidArgument)
		}

		sqlQueryTotal = `
      SELECT
        COUNT("event_project"."id")
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
        "event_project"."user_id",
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

	case *eventpb.Events_Request_ServerId:
		// setting for server events
		// use server_id and user_id
		if in.GetUserId() == "" {
			return nil, status.Error(codes.InvalidArgument, trace.MsgInvalidArgument)
		}

		sqlQueryTotal = `
      SELECT
        COUNT("event_server"."id")
      FROM
        "event_server"
        INNER JOIN "server" ON "event_server"."server_id" = "server"."id"
        INNER JOIN "project" ON "server"."project_id" = "project"."id"
      WHERE
        "event_server"."server_id" = $1
        AND "project"."owner_id" = $2
    `
		sqlQuery = `
      SELECT
        "event_server"."id",
        "event_server"."user_id",
        "event_server"."ip",
        "event_server"."event",
        "event_server"."section",
        "event_server"."created_at"
      FROM
        "event_server"
        INNER JOIN "server" ON "event_server"."server_id" = "server"."id"
        INNER JOIN "project" ON "server"."project_id" = "project"."id"
      WHERE
        "event_server"."server_id" = $1
        AND "project"."owner_id" = $2
    `
		args = append(args, in.GetServerId(), in.GetUserId())
	default:
		return nil, status.Error(codes.InvalidArgument, trace.MsgInvalidArgument)
	}

	// Total count for pagination
	err := h.DB.Conn.QueryRowContext(ctx, sqlQueryTotal, args...).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		return nil, trace.Error(err, log, nil)
	}
	if response.Total == 0 {
		return response, nil
	}

	// List records
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := h.DB.Conn.QueryContext(ctx, sqlQuery+sqlFooter, args...)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	for rows.Next() {
		var createdAt pgtype.Timestamp
		record := &eventpb.Event_Response{}

		err = rows.Scan(
			&record.Id,
			&record.UserId,
			&record.Ip,
			&record.Event,
			&record.Section,
			&createdAt,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		record.CreatedAt = timestamppb.New(createdAt.Time)
		response.Records = append(response.Records, record)
	}
	defer rows.Close()

	return response, nil
}

// Event is ...
func (h *Handler) Event(ctx context.Context, in *eventpb.Event_Request) (*eventpb.Event_Response, error) {
	var sqlQuery string
	var args []any

	if in.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, trace.MsgInvalidArgument)
	}

	switch in.GetId().(type) {
	case *eventpb.Event_Request_ProfileId:
		sqlQuery = `
      SELECT
        "id",
        "user_id",
        "user_agent",
        "ip",
        "event",
        "section",
        "data",
        "created_at"
      FROM
        "event_profile"
      WHERE
        "id" = $1
        AND "user_id" = $2
    `
		args = append(args, in.GetProfileId(), in.GetUserId())

	case *eventpb.Event_Request_ProjectId:
		sqlQuery = `
      SELECT
        "event_project"."project_id",
        "event_project"."user_id",
        "event_project"."user_agent",
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

	case *eventpb.Event_Request_ServerId:
		sqlQuery = `
      SELECT
        "event_server"."server_id",
        "event_server"."user_id",
        "event_server"."user_agent",
        "event_server"."ip",
        "event_server"."event",
        "event_server"."section",
        "event_server"."data",
        "event_server"."created_at"
      FROM
        "event_server"
        INNER JOIN "server" ON "event_server"."server_id" = "server"."id"
        INNER JOIN "project" ON "server"."project_id" = "project"."id"
      WHERE
        "event_server"."id" = $1
        AND "project"."owner_id" = $2
    `
		args = append(args, in.GetServerId(), in.GetUserId())
	default:
		return nil, status.Error(codes.InvalidArgument, trace.MsgInvalidArgument)
	}

	var createdAt pgtype.Timestamp

	response := &eventpb.Event_Response{}

	err := h.DB.Conn.QueryRowContext(ctx, sqlQuery, args...).Scan(
		&response.Id,
		&response.UserId,
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

	response.CreatedAt = timestamppb.New(createdAt.Time)
	return response, nil
}

// AddEvent is ...
func (h *Handler) AddEvent(ctx context.Context, in *eventpb.AddEvent_Request) (*eventpb.AddEvent_Response, error) {
	var sqlQuery, id, user_id string
	var section int32
	response := &eventpb.AddEvent_Response{}

	switch in.Section.(type) {
	case *eventpb.AddEvent_Request_Profile:
		sqlQuery = `
      INSERT INTO
        "event_profile" (
          "profile_id",
          "user_id",
          "user_agent",
          "ip",
          "event",
          "section",
          "data"
        )
      VALUES
        ($1, $2, $3, $4, $5, $6, $7)
      RETURNING
        id
    `
		section = int32(in.GetProfile().GetSection())
		id = in.GetProfile().Id
		user_id = id

	case *eventpb.AddEvent_Request_Project:
		if in.GetUserId() == "" {
			return nil, status.Error(codes.InvalidArgument, trace.MsgInvalidArgument)
		}
		sqlQuery = `
      INSERT INTO
        "event_project" (
          "project_id",
          "user_id",
          "user_agent",
          "ip",
          "event",
          "section",
          "data"
        )
      VALUES
        ($1, $2, $3, $4, $5, $6, $7)
      RETURNING
        id
    `
		section = int32(in.GetProject().Section)
		id = in.GetProject().Id
		user_id = in.GetUserId()

	case *eventpb.AddEvent_Request_Server:
		if in.GetUserId() == "" {
			return nil, status.Error(codes.InvalidArgument, trace.MsgInvalidArgument)
		}
		sqlQuery = `
      INSERT INTO
        "event_server" (
          "server_id",
          "user_id",
          "user_agent",
          "ip",
          "event",
          "section",
          "data"
        )
      VALUES
        ($1, $2, $3, $4, $5, $6, $7)
      RETURNING
        id
    `
		section = int32(in.GetServer().Section)
		id = in.GetServer().Id
		user_id = in.GetUserId()

	default:
		return nil, status.Error(codes.InvalidArgument, trace.MsgInvalidArgument)
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
		user_id,
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
