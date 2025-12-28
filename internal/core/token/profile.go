/*
IDEA:
1. Registration token
  1.1. an email with a registration token is sent (registration page)
  (profile_id is assigned)
  1.2. an email with registration confirmation is sent
2. Deletion token
(required presence of profile_id)
  2.1. an email with a deletion token and warning is sent (password entry page for deletion)
  2.2. an email with deletion confirmation is sent
*/

package token

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/werbot/werbot/internal/core/profile"
	profilepb "github.com/werbot/werbot/internal/core/profile/proto/profile"
	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/protoutils"
)

// ProfileTokens is ...
// this functionality can only be accessed with administrator rights
func (h *Handler) ProfileTokens(ctx context.Context, in *tokenmessage.ProfileTokens_Request) (*tokenmessage.ProfileTokens_Response, error) {
	// access only for admin or system request
	if !in.GetIsAdmin() {
		errGRPC := status.Error(codes.PermissionDenied, trace.MsgPermissionDenied)
		return nil, trace.Error(errGRPC, log, nil)
	}

	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	// check status value
	sqlWhereAction := getActionWhereClause(in.GetAction())
	sqlWhereStatus := getStatusWhereClause(in.GetStatus())
	response := &tokenmessage.ProfileTokens_Response{}

	// Total count for pagination
	totalQuery := postgres.SQLGluing(`
    SELECT COUNT(*)
    FROM "token"
    WHERE "section" = $1 AND "action" = $2
  `, sqlWhereAction, sqlWhereStatus)
	err := h.DB.Conn.QueryRowContext(ctx, totalQuery, tokenenum.Section_profile, in.GetAction()).Scan(&response.Total)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	if response.Total == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgTokenNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// List records
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	baseQuery := postgres.SQLGluing(`
    SELECT
      "id",
      "action",
      "status",
      "project_id",
      "scheme_id",
      "expired_at",
      "updated_at",
      "created_at"
    FROM "token"
    WHERE "section" = $1 AND "action" = $2
  `, sqlWhereAction, sqlWhereStatus, sqlFooter)

	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery, tokenenum.Section_profile, in.GetAction())
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	tokens, err := scanTokens(rows, in.GetLimit(), in.GetIsAdmin())
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	response.Tokens = tokens
	return response, nil
}

// AddTokenProfileReset is ...
func (h *Handler) AddTokenProfileReset(ctx context.Context, in *tokenmessage.AddTokenProfileReset_Request) (*tokenmessage.AddTokenProfileReset_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	// check profile
	profileDB := profile.Handler{DB: h.DB}
	_, err := profileDB.Profile(ctx, &profilepb.Profile_Request{
		ProfileId: in.GetProfileId(),
	})
	if err != nil {
		return nil, err
	}
	// ----

	fields := []string{"section", "action", "status", "data", "profile_id"}
	args := []any{tokenenum.Section_profile, tokenenum.Action_delete, tokenenum.Status_sent, []byte("{}"), in.GetProfileId()}

	if in.GetExpiredAt() != nil {
		fields = append(fields, "expired_at")
		args = append(args, in.GetExpiredAt().AsTime())
	}

	baseQuery, args := buildInsertQuery(fields, args)

	response := &tokenmessage.AddTokenProfileReset_Response{}
	err = h.DB.Conn.QueryRowContext(ctx, baseQuery, args...).Scan(&response.Token)
	if err != nil {
		errGRPC := status.Error(codes.InvalidArgument, trace.MsgFailedToAdd)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// TODO: send email message

	return response, nil
}

// AddTokenProfileRegistration is ...
func (h *Handler) AddTokenProfileRegistration(ctx context.Context, in *tokenmessage.AddTokenProfileRegistration_Request) (*tokenmessage.AddTokenProfileRegistration_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	data, err := protojson.Marshal(in.GetData())
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	fields := []string{"section", "action", "status", "data"}
	args := []any{tokenenum.Section_profile, tokenenum.Action_register, tokenenum.Status_sent, data}

	if in.GetExpiredAt() != nil {
		fields = append(fields, "expired_at")
		args = append(args, in.GetExpiredAt().AsTime())
	}

	baseQuery, args := buildInsertQuery(fields, args)

	response := &tokenmessage.AddTokenProfileRegistration_Response{}
	err = h.DB.Conn.QueryRowContext(ctx, baseQuery, args...).Scan(&response.Token)
	if err != nil {
		errGRPC := status.Error(codes.InvalidArgument, trace.MsgFailedToAdd)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// TODO: send email message

	return response, nil
}

// AddTokenProfileDelete is ...
func (h *Handler) AddTokenProfileDelete(ctx context.Context, in *tokenmessage.AddTokenProfileDelete_Request) (*tokenmessage.AddTokenProfileDelete_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	// check profile
	profileDB := profile.Handler{DB: h.DB}
	_, err := profileDB.Profile(ctx, &profilepb.Profile_Request{
		ProfileId: in.GetProfileId(),
	})
	if err != nil {
		return nil, err
	}
	// ----

	fields := []string{"section", "action", "status", "profile_id", "data"}
	args := []any{tokenenum.Section_profile, tokenenum.Action_delete, tokenenum.Status_sent, in.GetProfileId(), []byte("{}")}

	if in.GetExpiredAt() != nil {
		fields = append(fields, "expired_at")
		args = append(args, in.GetExpiredAt().AsTime())
	}

	baseQuery, args := buildInsertQuery(fields, args)

	response := &tokenmessage.AddTokenProfileDelete_Response{}
	err = h.DB.Conn.QueryRowContext(ctx, baseQuery, args...).Scan(&response.Token)
	if err != nil {
		errGRPC := status.Error(codes.InvalidArgument, trace.MsgFailedToAdd)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// TODO: send email message

	return response, nil
}

// UpdateProfileToken is ...
func (h *Handler) UpdateProfileToken(ctx context.Context, in *tokenmessage.UpdateProfileToken_Request) (*tokenmessage.UpdateProfileToken_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	if !in.GetIsAdmin() && (in.GetStatus() == tokenenum.Status_status_unspecified || in.GetStatus() == tokenenum.Status_deleted || in.GetStatus() == tokenenum.Status_archived) {
		return nil, trace.Error(status.Error(codes.NotFound, trace.MsgStatusNotFound), log, nil)
	}

	// get core data for token
	tokenData, err := h.Token(ctx, &tokenmessage.Token_Request{
		IsAdmin: true,
		Token:   in.GetToken(),
	})
	if err != nil {
		return nil, trace.Error(status.Error(codes.NotFound, trace.MsgTokenNotFound), log, nil)
	}
	if !in.GetIsAdmin() && tokenData.GetStatus() == tokenenum.Status_done {
		return nil, trace.Error(status.Error(codes.PermissionDenied, trace.MsgPermissionDenied), log, nil)
	}

	// SQL constructor
	query := `UPDATE "token" SET "status" = $2`
	args := []any{in.GetToken(), in.GetStatus()}
	phCount := 3 // Start from $3 since $1 and $2 are already used

	// helper function to add conditions to the SQL statement
	addCondition := func(fieldName, value string) error {
		if value == "" {
			return trace.Error(status.Error(codes.InvalidArgument, trace.MsgInvalidArgument), log, fmt.Sprintf("%s: value is required", fieldName))
		}
		query = postgres.SQLGluingOptions{Separator: ","}.SQLGluing(query, fmt.Sprintf(`"%s" = $%d`, fieldName, phCount))
		args = append(args, value)
		phCount++
		return nil
	}

	// 1-2-1: registration of a new profile and sending a notification by email
	if tokenData.GetAction() == tokenenum.Action_register && in.GetStatus() == tokenenum.Status_done {
		if err := addCondition("profile_id", in.GetProfileId()); err != nil {
			return nil, err
		}
	}

	query = postgres.SQLGluing(query, `WHERE "id" = $1`)

	result, err := h.DB.Conn.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, handleSQLError(err)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		return nil, trace.Error(status.Error(codes.NotFound, trace.MsgTokenNotFound), log, nil)
	}

	// TODO: send email message if need
	// use data from tokenData

	return &tokenmessage.UpdateProfileToken_Response{}, nil
}
