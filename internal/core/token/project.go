/*
IDEA:
1. Invitation to join a project (adding a member) (profile_id and project_id are required)
  1.1. An email is sent with a token for joining the project (login page if the user is not logged in)
  1.2. An email is sent confirming addition to the project
2. Invitation to register a profile (adding a member) and subsequent addition to the project (project_id is required)
  2.1. An email is sent with a token for profile registration with automatic linking to the project afterwards (new profile registration page)
  (profile_id is assigned)
  2.2. An email is sent confirming addition to the project
*/

package token

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/protoutils"
)

// ProjectTokens retrieves a list of project tokens filtered by action and optional status
// Requires owner_id and project_id to verify project ownership
// Returns paginated list of tokens with total count
func (h *Handler) ProjectTokens(ctx context.Context, in *tokenmessage.ProjectTokens_Request) (*tokenmessage.ProjectTokens_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	// check status value
	sqlWhereAction := getActionWhereClause(in.GetAction())
	sqlWhereStatus := getStatusWhereClause(in.GetStatus())
	response := &tokenmessage.ProjectTokens_Response{}

	// Total count for pagination
	totalQuery := postgres.SQLGluing(`
    SELECT COUNT(*)
    FROM "token"
      INNER JOIN "project" ON "project"."id" = "token"."project_id"
    WHERE
      "token"."section" = $1
      AND "project"."owner_id" = $2
      AND "token"."project_id" = $3
  `, sqlWhereAction, sqlWhereStatus)
	err := h.DB.Conn.QueryRowContext(ctx, totalQuery, tokenenum.Section_project, in.GetOwnerId(), in.GetProjectId()).Scan(&response.Total)
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
      INNER JOIN "project" ON "project"."id" = "token"."project_id"
    WHERE
      "token"."section" = $1
      AND "project"."owner_id" = $2
      AND "token"."project_id" = $3
  `, sqlWhereAction, sqlWhereStatus, sqlFooter)

	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery, tokenenum.Section_project, in.GetOwnerId(), in.GetProjectId())
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

// AddTokenProjectMember creates a project member invitation token
// Supports two modes: existing profile (by profile_id) or new profile registration (by email/name/surname)
// Validates project ownership and profile existence
// Supports custom expiration time via expired_at parameter
// Returns the created token ID
func (h *Handler) AddTokenProjectMember(ctx context.Context, in *tokenmessage.AddTokenProjectMember_Request) (*tokenmessage.AddTokenProjectMember_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	// check project exists and access
	var projectExists bool
	err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT EXISTS(
        SELECT 1 FROM "project"
        WHERE "id" = $1 AND "owner_id" = $2
      )
    `, in.GetProjectId(), in.GetOwnerId()).Scan(&projectExists)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	if !projectExists {
		errGRPC := status.Error(codes.NotFound, trace.MsgProjectNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	fields := []string{"section", "action", "status", "project_id"}
	args := []any{tokenenum.Section_project, tokenenum.Action_request, tokenenum.Status_sent}

	switch in.GetProfile().(type) {
	case *tokenmessage.AddTokenProjectMember_Request_CreateNewProfile:
		profileData := in.GetCreateNewProfile()
		data, err := protojson.Marshal(profileData)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		// check if profile exists by email
		profileDataByEmail, err := h.GetProfileDataByEmail(ctx, profileData.Email)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}
		profileID := profileDataByEmail.ID

		fields = append(fields, "data")
		args = append(args, data)
		if profileID != "" {
			fields = append(fields, "profile_id")
			args = append(args, profileID)
		}

	case *tokenmessage.AddTokenProjectMember_Request_ProfileId:
		profileID := in.GetProfileId()
		// check profile exists
		var profileExists bool
		err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT EXISTS(SELECT 1 FROM "profile" WHERE "id" = $1)
    `, profileID).Scan(&profileExists)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}
		if !profileExists {
			errGRPC := status.Error(codes.NotFound, trace.MsgProfileNotFound)
			return nil, trace.Error(errGRPC, log, nil)
		}

		fields = append(fields, "profile_id")
		args = append(args, profileID)
	}

	if in.GetExpiredAt() != nil {
		fields = append(fields, "expired_at")
		args = append(args, in.GetExpiredAt().AsTime())
	}

	baseQuery, args := buildInsertQuery(fields, args)

	response := &tokenmessage.AddTokenProjectMember_Response{}
	err = h.DB.Conn.QueryRowContext(ctx, baseQuery, args...).Scan(&response.Token)
	if err != nil {
		errGRPC := status.Error(codes.InvalidArgument, trace.MsgFailedToAdd)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// TODO: send email message

	return response, nil
}

// UpdateProjectToken updates project token status
// Validates token status transitions based on user permissions (admin vs regular user)
// For member invitation tokens, requires profile_id when updating to done status
// Returns error if validation fails or token not found
func (h *Handler) UpdateProjectToken(ctx context.Context, in *tokenmessage.UpdateProjectToken_Request) (*tokenmessage.UpdateProjectToken_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	// get core data for token
	tokenData, err := h.Token(ctx, &tokenmessage.Token_Request{
		IsAdmin: true,
		Token:   in.GetToken(),
	})
	if err != nil {
		return nil, trace.Error(status.Error(codes.NotFound, trace.MsgTokenNotFound), log, nil)
	}

	// Validate token for update using common validation function
	if err := ValidateTokenForUpdate(in.GetIsAdmin(), tokenData.GetStatus(), in.GetStatus()); err != nil {
		return nil, trace.Error(err, log, nil)
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
	// updating token related to project member
	if tokenData.GetAction() == tokenenum.Action_request &&
		tokenData.GetProfileId() == "" &&
		in.GetStatus() == tokenenum.Status_done {
		if err := addCondition("profile_id", in.GetProfileId()); err != nil {
			return nil, err
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

	return &tokenmessage.UpdateProjectToken_Response{}, nil
}
