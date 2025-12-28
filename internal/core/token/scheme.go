/*
IDEA:
1. Invitation to add a scheme to the project (project_id is required)
  1.1. an email with a token for adding the scheme is sent (page of the scheme addition form)
  1.2. an email confirming the addition of the scheme is sent

2. Invitation for one-time web access to the scheme (ssh console, applications)
  (scheme_id is required, profile_id is optional if there is no profile association)
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
	"github.com/werbot/werbot/internal/core/project"
	projectpb "github.com/werbot/werbot/internal/core/project/proto/project"
	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/protoutils"
)

func (h *Handler) SchemeTokens(ctx context.Context, in *tokenmessage.SchemeTokens_Request) (*tokenmessage.SchemeTokens_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	// check status value
	sqlWhereAction := getActionWhereClause(in.GetAction())
	sqlWhereStatus := getStatusWhereClause(in.GetStatus())
	response := &tokenmessage.SchemeTokens_Response{}

	// Total count for pagination
	totalQuery := postgres.SQLGluing(`
    SELECT COUNT(*)
    FROM
      "token"
      INNER JOIN "scheme" ON "token"."scheme_id" = "scheme". "id"
      INNER JOIN "project" ON "scheme"."project_id" = "project". "id"
    WHERE
      "token". "section" = $1
      AND "project"."owner_id" = $2
      AND "token"."scheme_id" = $3
  `, sqlWhereAction, sqlWhereStatus)
	err := h.DB.Conn.QueryRowContext(ctx, totalQuery, tokenenum.Section_scheme, in.GetOwnerId(), in.GetSchemeId()).Scan(&response.Total)
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
      "token"."id",
      "token"."action",
      "token"."status",
      "token"."profile_id",
      "token"."scheme_id",
      "token"."expired_at",
      "token"."updated_at",
      "token"."created_at"
    FROM
      "token"
      INNER JOIN "scheme" ON "token"."scheme_id" = "scheme". "id"
      INNER JOIN "project" ON "scheme"."project_id" = "project". "id"
    WHERE
      "token". "section" = $1
      AND "project"."owner_id" = $2
      AND "token"."scheme_id" = $3
  `, sqlWhereAction, sqlWhereStatus, sqlFooter)

	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery, tokenenum.Section_scheme, in.GetOwnerId(), in.GetSchemeId())
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

// AddTokenSchemeAdd is ...
func (h *Handler) AddTokenSchemeAdd(ctx context.Context, in *tokenmessage.AddTokenSchemeAdd_Request) (*tokenmessage.AddTokenSchemeAdd_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	// check access for add
	projectDB := project.Handler{DB: h.DB}
	_, err := projectDB.Project(ctx, &projectpb.Project_Request{
		OwnerId:   in.GetOwnerId(),
		ProjectId: in.GetProjectId(),
	})
	if err != nil {
		return nil, err
	}
	// ----

	fields := []string{"section", "action", "status", "project_id"}
	args := []any{tokenenum.Section_scheme, tokenenum.Action_add, tokenenum.Status_sent, in.GetProjectId()}

	profileDB := profile.Handler{DB: h.DB, Worker: h.Worker}
	var metaData *tokenmessage.MetaDataProfile

	switch in.GetData().(type) {
	case *tokenmessage.AddTokenSchemeAdd_Request_Email:
		profileData, err := profileDB.ProfileByEmail(ctx, &profilepb.ProfileByEmail_Request{Email: in.GetEmail()})
		if err != nil && status.Convert(err).Code() != codes.NotFound {
			return nil, err
		}

		if profileData.GetProfileId() != "" {
			fields = append(fields, "profile_id")
			args = append(args, profileData.GetProfileId())
		}

		metaData = &tokenmessage.MetaDataProfile{
			Email: in.GetEmail(),
		}

	case *tokenmessage.AddTokenSchemeAdd_Request_ProfileId:
		profileData, err := profileDB.Profile(ctx, &profilepb.Profile_Request{ProfileId: in.GetProfileId()})
		if err != nil {
			return nil, err
		}

		fields = append(fields, "profile_id")
		args = append(args, in.GetProfileId())

		metaData = &tokenmessage.MetaDataProfile{
			Name:    profileData.GetName(),
			Surname: profileData.GetSurname(),
			Email:   profileData.GetEmail(),
		}
	}

	if metaData != nil {
		fields = append(fields, "data")
		data, err := protojson.Marshal(metaData)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}
		args = append(args, data)
	}

	if in.GetExpiredAt() != nil {
		fields = append(fields, "expired_at")
		args = append(args, in.GetExpiredAt().AsTime())
	}

	baseQuery, args := buildInsertQuery(fields, args)

	response := &tokenmessage.AddTokenSchemeAdd_Response{}
	err = h.DB.Conn.QueryRowContext(ctx, baseQuery, args...).Scan(&response.Token)
	if err != nil {
		errGRPC := status.Error(codes.InvalidArgument, trace.MsgFailedToAdd)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// TODO: send email message

	return response, nil
}

// AddTokenSchemeAccess is ...
func (h *Handler) AddTokenSchemeAccess(ctx context.Context, in *tokenmessage.AddTokenSchemeAccess_Request) (*tokenmessage.AddTokenSchemeAccess_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	// check access for add
	var schemeCount int
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM
      "scheme"
      INNER JOIN "project" ON "scheme"."project_id" = "project"."id"
    WHERE
      "project"."owner_id" = $1
      AND "scheme"."id" = $2
  `, in.GetOwnerId(), in.GetSchemeId()).Scan(&schemeCount)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	if schemeCount == 0 {
		return nil, trace.Error(status.Error(codes.NotFound, trace.MsgNotFound), log, nil)
	}
	// ----

	fields := []string{"section", "action", "status", "scheme_id"}
	args := []any{tokenenum.Section_scheme, tokenenum.Action_request, tokenenum.Status_sent, in.GetSchemeId()}

	var metaData *tokenmessage.MetaDataProfile
	profile := profile.Handler{DB: h.DB, Worker: h.Worker}

	switch in.GetData().(type) {
	case *tokenmessage.AddTokenSchemeAccess_Request_Email:
		profileData, err := profile.ProfileByEmail(ctx, &profilepb.ProfileByEmail_Request{Email: in.GetEmail()})
		if err != nil && status.Convert(err).Code() != codes.NotFound {
			return nil, err
		}
		if profileData.GetProfileId() != "" {
			fields = append(fields, "profile_id")
			args = append(args, profileData.GetProfileId())
		}

		metaData = &tokenmessage.MetaDataProfile{
			Email: in.GetEmail(),
		}

	case *tokenmessage.AddTokenSchemeAccess_Request_ProfileId:
		profileData, err := profile.Profile(ctx, &profilepb.Profile_Request{ProfileId: in.GetProfileId()})
		if err != nil {
			return nil, err
		}

		fields = append(fields, "profile_id")
		args = append(args, in.GetProfileId())

		metaData = &tokenmessage.MetaDataProfile{
			Name:    profileData.GetName(),
			Surname: profileData.GetSurname(),
			Email:   profileData.GetEmail(),
		}
	}

	if metaData != nil {
		fields = append(fields, "data")
		data, err := protojson.Marshal(metaData)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}
		args = append(args, data)
	}

	if in.GetExpiredAt() != nil {
		fields = append(fields, "expired_at")
		args = append(args, in.GetExpiredAt().AsTime())
	}

	baseQuery, args := buildInsertQuery(fields, args)

	response := &tokenmessage.AddTokenSchemeAccess_Response{}
	err = h.DB.Conn.QueryRowContext(ctx, baseQuery, args...).Scan(&response.Token)
	if err != nil {
		errGRPC := status.Error(codes.InvalidArgument, trace.MsgFailedToAdd)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// TODO: send email message

	return response, nil
}

// UpdateSchemeToken is ...
func (h *Handler) UpdateSchemeToken(ctx context.Context, in *tokenmessage.UpdateSchemeToken_Request) (*tokenmessage.UpdateSchemeToken_Response, error) {
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

	// -----
	// adding a new scheme to the project
	if tokenData.GetAction() == tokenenum.Action_add &&
		tokenData.GetSchemeId() == "" &&
		in.GetStatus() == tokenenum.Status_done {
		if err := addCondition("scheme_id", in.GetSchemeId()); err != nil {
			return nil, err
		}
		// if regestered profile
		if in.GetProfileId() != "" {
			if err := addCondition("profile_id", in.GetProfileId()); err != nil {
				return nil, err
			}
		}
	}
	// -----

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

	return &tokenmessage.UpdateSchemeToken_Response{}, nil
}
