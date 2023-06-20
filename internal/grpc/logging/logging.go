package logging

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	loggingpb "github.com/werbot/werbot/internal/grpc/logging/proto"
	"github.com/werbot/werbot/internal/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ListRecords is ...
func (h *Handler) ListRecords(ctx context.Context, in *loggingpb.ListRecords_Request) (*loggingpb.ListRecords_Response, error) {
	response := new(loggingpb.ListRecords_Response)

	loggerType := in.GetLogger()
	tableName, ok := loggerTable[loggerType]
	if !ok {
		return nil, trace.Error(codes.InvalidArgument)
	}

	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	sqlQuery := fmt.Sprintf(`SELECT "id", "user_id", "user_agent", "ip", "event", "data"
    FROM "%s" WHERE "%s_id"=$1 %s`,
		tableName,
		strings.ToLower(loggerType.String()),
		sqlFooter,
	)

	rows, err := h.DB.Conn.QueryContext(ctx, sqlQuery,
		in.GetRecordId(),
	)

	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	for rows.Next() {
		var created pgtype.Timestamp
		record := new(loggingpb.Record_Response)

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

	// Total count for pagination
	sqlQueryTotal := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE "%s_id"=$1`,
		tableName,
		strings.ToLower(loggerType.String()))
	err = h.DB.Conn.QueryRowContext(ctx, sqlQueryTotal,
		in.GetRecordId(),
	).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	return response, nil
}

// Record is ...
func (h *Handler) Record(ctx context.Context, in *loggingpb.Record_Request) (*loggingpb.Record_Response, error) {
	var created pgtype.Timestamp
	//var metaData pgtype.JSONBCodec
	response := new(loggingpb.Record_Response)

	loggerType := in.GetLogger()
	tableName, ok := loggerTable[loggerType]
	if !ok {
		return nil, trace.Error(codes.InvalidArgument)
	}

	sqlQuery := fmt.Sprintf(`SELECT "%s_id", "user_id", "user_agent", "ip", "event", "data"
    FROM "%s" WHERE "id"=$1`,
		strings.ToLower(loggerType.String()),
		tableName,
	)

	err := h.DB.Conn.QueryRowContext(ctx, sqlQuery,
		in.GetRecordId(),
	).Scan(&response.Id,
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

// AddRecord is ...
func (h *Handler) AddRecord(ctx context.Context, in *loggingpb.AddRecord_Request) (*loggingpb.AddRecord_Response, error) {
	response := new(loggingpb.AddRecord_Response)

	loggerType := in.GetLogger()
	tableName, ok := loggerTable[loggerType]
	if !ok {
		return nil, trace.Error(codes.InvalidArgument)
	}

	sqlQuery := fmt.Sprintf(`INSERT INTO "%s" ("%s_id", "user_id", "user_agent", "ip", "event", "data")
    VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		tableName,
		strings.ToLower(loggerType.String()))

	tx, err := h.DB.Conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgTransactionCreateError)
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, sqlQuery,
		in.GetId(),
		in.GetUserId(),
		in.GetUserAgent(),
		in.GetIp(),
		in.GetEvent(),
		"{}", // in.GetMetaData()
	).Scan(&response.RecordId)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToAdd)
	}

	if err := tx.Commit(); err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgTransactionCommitError)
	}

	return response, nil
}
