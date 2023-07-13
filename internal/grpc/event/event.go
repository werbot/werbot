package event

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventpb "github.com/werbot/werbot/internal/grpc/event/proto"
	"github.com/werbot/werbot/internal/trace"
)

// Events is ...
func (h *Handler) Events(ctx context.Context, in *eventpb.Events_Request) (*eventpb.Events_Response, error) {
	var sqlQuery, sqlQueryTotal string
	var args []any
	response := new(eventpb.Events_Response)

	switch in.GetId().(type) {
	// setting for profile events
	// use only profile_id
	case *eventpb.Events_Request_ProfileId:
		sqlQueryTotal = `SELECT COUNT("id") FROM "event_profile" WHERE "profile_id"=$1`
		sqlQuery = `SELECT "id", "user_id", "user_agent", "ip", "event", "data"
      FROM "event_profile" WHERE "profile_id"=$1`
		args = append(args, in.GetProfileId())

	case *eventpb.Events_Request_ProjectId:
		// setting for project events
		// use profile_id and user_id
		if in.GetUserId() == "" {
			return nil, trace.Error(codes.InvalidArgument)
		}

		sqlQueryTotal = `SELECT COUNT("event_project"."id")
      FROM
        "event_project"
        INNER JOIN "project" ON "event_project"."project_id" = "project"."id"
      WHERE
        "event_project"."project_id" = $1
        AND "project"."owner_id" = $2`
		sqlQuery = `SELECT
        "event_project"."id",
        "event_project"."user_id",
        "event_project"."user_agent",
        "event_project"."ip",
        "event_project"."event",
        "event_project"."data"
      FROM "event_project"
        INNER JOIN "project" ON "event_project"."project_id" = "project"."id"
      WHERE
        "event_project"."project_id" = $1
        AND "project"."owner_id" = $2`
		args = append(args, in.GetProjectId(), in.GetUserId())

	case *eventpb.Events_Request_ServerId:
		// setting for server events
		// use server_id and user_id
		if in.GetUserId() == "" {
			return nil, trace.Error(codes.InvalidArgument)
		}

		sqlQueryTotal = `SELECT COUNT("event_server"."id")
      FROM
        "event_server"
        INNER JOIN "server" ON "event_server"."server_id" = "server"."id"
        INNER JOIN "project" ON "server"."project_id" = "project"."id"
      WHERE
        "event_server"."server_id" = $1
        AND "project"."owner_id" = $2`
		sqlQuery = `SELECT
        "event_server"."id",
        "event_server"."user_id",
        "event_server"."user_agent",
        "event_server"."ip",
        "event_server"."event",
        "event_server"."data"
      FROM
        "event_server"
        INNER JOIN "server" ON "event_server"."server_id" = "server"."id"
        INNER JOIN "project" ON "server"."project_id" = "project"."id"
      WHERE
        "event_server"."server_id" = $1
        AND "project"."owner_id" = $2`
		args = append(args, in.GetServerId(), in.GetUserId())
	default:
		return nil, trace.Error(codes.InvalidArgument)
	}

	// Total count for pagination
	err := h.DB.Conn.QueryRowContext(ctx, sqlQueryTotal, args...).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		return nil, trace.ErrorAborted(err, h.Log)
	}
	if response.Total == 0 {
		return response, nil
	}

	// List records
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := h.DB.Conn.QueryContext(ctx, sqlQuery+sqlFooter, args...)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	for rows.Next() {
		var created pgtype.Timestamp
		record := new(eventpb.Event_Response)

		err = rows.Scan(&record.Id,
			&record.UserId,
			&record.UserAgent,
			&record.Ip,
			&record.Event,
			&record.MetaData,
		)
		if err != nil {
			return nil, trace.ErrorAborted(err, h.Log)
		}

		record.Created = timestamppb.New(created.Time)
		response.Records = append(response.Records, record)
	}
	defer rows.Close()

	return response, nil
}

// Event is ...
func (h *Handler) Event(ctx context.Context, in *eventpb.Event_Request) (*eventpb.Event_Response, error) {
	var sqlQuery string
	var args []any
	var created pgtype.Timestamp
	response := new(eventpb.Event_Response)

	if in.GetUserId() == "" {
		return nil, trace.Error(codes.InvalidArgument)
	}

	switch in.GetId().(type) {
	case *eventpb.Event_Request_ProfileId:
		sqlQuery = `SELECT "profile_id", "user_id", "user_agent", "ip", "event", "data"
      FROM "event_profile" WHERE "id"=$1 AND "user_id"=$2`
		args = append(args, in.GetProfileId(), in.GetUserId())

	case *eventpb.Event_Request_ProjectId:
		sqlQuery = `SELECT
        "event_project"."project_id",
        "event_project"."user_id",
        "event_project"."user_agent",
        "event_project"."ip",
        "event_project"."event",
        "event_project"."data"
      FROM "event_project"
        INNER JOIN "project" ON "event_project"."project_id" = "project"."id"
      WHERE
        "event_project"."id" = $1
        AND "project"."owner_id" = $2`
		args = append(args, in.GetProjectId(), in.GetUserId())

	case *eventpb.Event_Request_ServerId:
		sqlQuery = `SELECT
        "event_server"."server_id",
        "event_server"."user_id",
        "event_server"."user_agent",
        "event_server"."ip",
        "event_server"."event",
        "event_server"."data"
      FROM
        "event_server"
        INNER JOIN "server" ON "event_server"."server_id" = "server"."id"
        INNER JOIN "project" ON "server"."project_id" = "project"."id"
      WHERE
        "event_server"."id" = $1
        AND "project"."owner_id" = $2`
		args = append(args, in.GetServerId(), in.GetUserId())
	default:
		return nil, trace.Error(codes.InvalidArgument)
	}

	err := h.DB.Conn.QueryRowContext(ctx, sqlQuery, args...).Scan(&response.Id,
		&response.UserId,
		&response.UserAgent,
		&response.Ip,
		&response.Event,
		&response.MetaData,
	)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	response.Created = timestamppb.New(created.Time)
	return response, nil
}

// AddEvent is ...
func (h *Handler) AddEvent(ctx context.Context, in *eventpb.AddEvent_Request) (*eventpb.AddEvent_Response, error) {
	var sqlQuery, id, user_id string
	response := new(eventpb.AddEvent_Response)

	switch in.Id.(type) {
	case *eventpb.AddEvent_Request_ProfileId:
		sqlQuery = `INSERT INTO "event_profile" ("profile_id", "user_id", "user_agent", "ip", "event", "data")
      VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
		id = in.GetProfileId()
		user_id = id

	case *eventpb.AddEvent_Request_ProjectId:
		if in.GetUserId() == "" {
			return nil, trace.Error(codes.InvalidArgument)
		}
		sqlQuery = `INSERT INTO "event_project" ("project_id", "user_id", "user_agent", "ip", "event", "data")
      VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
		id = in.GetProjectId()
		user_id = in.GetUserId()

	case *eventpb.AddEvent_Request_ServerId:
		if in.GetUserId() == "" {
			return nil, trace.Error(codes.InvalidArgument)
		}
		sqlQuery = `INSERT INTO "event_server" ("server_id", "user_id", "user_agent", "ip", "event", "data")
      VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
		id = in.GetServerId()
		user_id = in.GetUserId()

	default:
		return nil, trace.Error(codes.InvalidArgument)
	}

	tx, err := h.DB.Conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgTransactionCreateError)
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
		json,
	).Scan(&response.RecordId)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToAdd)
	}

	if err := tx.Commit(); err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgTransactionCommitError)
	}

	return response, nil
}
