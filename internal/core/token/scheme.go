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
	"database/sql"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/protoutils"
)

// SchemeTokens retrieves a list of scheme tokens filtered by action and optional status
// Requires owner_id and scheme_id to verify scheme ownership through project
// Returns paginated list of tokens with total count
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
      "token"."section",
      "token"."action",
      "token"."status",
      "token"."profile_id",
      "token"."scheme_id",
      COALESCE(
        NULLIF("token"."data", '{}'::jsonb),
        jsonb_strip_nulls(jsonb_build_object('email', "profile"."email"))
      ) AS "data",
      "token"."expired_at",
      "token"."updated_at",
      "token"."created_at"
    FROM
      "token"
      INNER JOIN "scheme" ON "token"."scheme_id" = "scheme". "id"
      INNER JOIN "project" ON "scheme"."project_id" = "project". "id"
      LEFT JOIN "profile" ON "profile"."id" = "token"."profile_id"
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

// AddTokenSchemeAdd creates a token for adding a scheme to a project
// Supports two modes: existing profile (by profile_id) or email-based invitation
// Validates project ownership and profile existence
// Supports custom expiration time via expired_at parameter
// Returns the created token ID
func (h *Handler) AddTokenSchemeAdd(ctx context.Context, in *tokenmessage.AddTokenSchemeAdd_Request) (*tokenmessage.AddTokenSchemeAdd_Response, error) {
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

	fields := []string{fieldSection, fieldAction, fieldStatus, "project_id"}
	args := []any{tokenenum.Section_scheme, tokenenum.Action_add, tokenenum.Status_sent, in.GetProjectId()}

	var metaData *tokenmessage.MetaDataProfile

	switch in.GetData().(type) {
	case *tokenmessage.AddTokenSchemeAdd_Request_Email:
		// check if profile exists by email
		profileDataByEmail, err := h.GetProfileDataByEmail(ctx, in.GetEmail())
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if profileDataByEmail.Exists {
			fields = append(fields, "profile_id")
			args = append(args, profileDataByEmail.ID)
			metaData = &tokenmessage.MetaDataProfile{
				Name:    profileDataByEmail.Name,
				Surname: profileDataByEmail.Surname,
				Email:   in.GetEmail(),
			}
		} else {
			metaData = &tokenmessage.MetaDataProfile{
				Email: in.GetEmail(),
			}
		}

	case *tokenmessage.AddTokenSchemeAdd_Request_ProfileId:
		// check profile exists and get data
		var name, surname, email string
		err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT "name", "surname", "email" FROM "profile" WHERE "id" = $1
    `, in.GetProfileId()).Scan(&name, &surname, &email)
		if err != nil {
			if err == sql.ErrNoRows {
				errGRPC := status.Error(codes.NotFound, trace.MsgProfileNotFound)
				return nil, trace.Error(errGRPC, log, nil)
			}
			return nil, trace.Error(err, log, nil)
		}

		fields = append(fields, "profile_id")
		args = append(args, in.GetProfileId())

		metaData = &tokenmessage.MetaDataProfile{
			Name:    name,
			Surname: surname,
			Email:   email,
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

// AddTokenSchemeAccess creates a token for one-time web access to a scheme
// Supports two modes: existing profile (by profile_id) or email-based invitation
// Validates scheme ownership through project
// Supports custom expiration time via expired_at parameter
// Returns the created token ID
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
		return nil, trace.Error(status.Error(codes.NotFound, trace.MsgSchemeNotFound), log, nil)
	}
	// ----

	fields := []string{fieldSection, fieldAction, fieldStatus, "scheme_id"}
	args := []any{tokenenum.Section_scheme, tokenenum.Action_request, tokenenum.Status_sent, in.GetSchemeId()}

	var metaData *tokenmessage.MetaDataProfile

	switch in.GetData().(type) {
	case *tokenmessage.AddTokenSchemeAccess_Request_Email:
		// check if profile exists by email
		profileDataByEmail, err := h.GetProfileDataByEmail(ctx, in.GetEmail())
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if profileDataByEmail.Exists {
			fields = append(fields, "profile_id")
			args = append(args, profileDataByEmail.ID)
			metaData = &tokenmessage.MetaDataProfile{
				Name:    profileDataByEmail.Name,
				Surname: profileDataByEmail.Surname,
				Email:   in.GetEmail(),
			}
		} else {
			metaData = &tokenmessage.MetaDataProfile{
				Email: in.GetEmail(),
			}
		}

	case *tokenmessage.AddTokenSchemeAccess_Request_ProfileId:
		// check profile exists and get data
		var name, surname, email string
		err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT "name", "surname", "email" FROM "profile" WHERE "id" = $1
    `, in.GetProfileId()).Scan(&name, &surname, &email)
		if err != nil {
			if err == sql.ErrNoRows {
				errGRPC := status.Error(codes.NotFound, trace.MsgProfileNotFound)
				return nil, trace.Error(errGRPC, log, nil)
			}
			return nil, trace.Error(err, log, nil)
		}

		fields = append(fields, "profile_id")
		args = append(args, in.GetProfileId())

		metaData = &tokenmessage.MetaDataProfile{
			Name:    name,
			Surname: surname,
			Email:   email,
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

// UpdateSchemeToken updates scheme token status
// Validates token status transitions based on user permissions (admin vs regular user)
// For scheme add tokens, requires scheme_id when updating to done status
// Optionally requires profile_id if profile was registered
// Returns error if validation fails or token not found
func (h *Handler) UpdateSchemeToken(ctx context.Context, in *tokenmessage.UpdateSchemeToken_Request) (*tokenmessage.UpdateSchemeToken_Response, error) {
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

	profileID := tokenData.GetProfileId()

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

	// accepting a scheme access invite
	if tokenData.GetAction() == tokenenum.Action_request &&
		in.GetStatus() == tokenenum.Status_done &&
		tokenData.GetProfileId() == "" {
		if err := addCondition("profile_id", in.GetProfileId()); err != nil {
			return nil, err
		}
		profileID = in.GetProfileId()
	}
	// -----

	query = postgres.SQLGluing(query, `WHERE "id" = $1`)

	tx, err := h.DB.Conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCreateError)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, handleSQLError(err)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		return nil, trace.Error(status.Error(codes.NotFound, trace.MsgTokenNotFound), log, nil)
	}

	// Accepting a scheme invite also grants project and scheme membership.
	if tokenData.GetAction() == tokenenum.Action_request && in.GetStatus() == tokenenum.Status_done {
		if profileID == "" {
			return nil, trace.Error(status.Error(codes.InvalidArgument, trace.MsgInvalidArgument), log, "profile_id: value is required")
		}

		schemeID := tokenData.GetSchemeId()
		if schemeID == "" {
			schemeID = in.GetSchemeId()
		}
		if schemeID == "" {
			return nil, trace.Error(status.Error(codes.InvalidArgument, trace.MsgInvalidArgument), log, "scheme_id: value is required")
		}

		if err := addSchemeMemberForToken(ctx, tx, schemeID, profileID); err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToAdd)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, trace.Error(err, log, trace.MsgTransactionCommitError)
	}

	// TODO: send email message if need
	// use data from tokenData

	return &tokenmessage.UpdateSchemeToken_Response{}, nil
}

// schemeConnectTTL is the lifetime of a one-time telnet listener token.
const schemeConnectTTL = 5 * time.Minute

// AddTokenSchemeConnect creates a one-time token for connecting to a telnet
// scheme through the avocado telnet listener.
func (h *Handler) AddTokenSchemeConnect(ctx context.Context, in *tokenmessage.AddTokenSchemeConnect_Request) (*tokenmessage.AddTokenSchemeConnect_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	if _, err := h.activeSchemeMemberID(ctx, in.GetProfileId(), in.GetSchemeId()); err != nil {
		if err == sql.ErrNoRows {
			exists, existsErr := h.hasSchemeMembership(ctx, in.GetProfileId(), in.GetSchemeId())
			if existsErr != nil {
				return nil, trace.Error(existsErr, log, nil)
			}
			if exists {
				return nil, trace.Error(status.Error(codes.NotFound, trace.MsgSchemeNotFound), log, nil)
			}
			if _, err := h.enrollProjectOwner(ctx, in.GetProfileId(), in.GetSchemeId()); err != nil {
				if err == sql.ErrNoRows {
					return nil, trace.Error(status.Error(codes.NotFound, trace.MsgSchemeNotFound), log, nil)
				}
				return nil, trace.Error(err, log, nil)
			}
		} else {
			return nil, trace.Error(err, log, nil)
		}
	}

	expiresAt := time.Now().UTC().Add(schemeConnectTTL)
	fields := []string{fieldSection, fieldAction, fieldStatus, "profile_id", "scheme_id", "expired_at"}
	args := []any{
		tokenenum.Section_scheme,
		tokenenum.Action_connect,
		tokenenum.Status_sent,
		in.GetProfileId(),
		in.GetSchemeId(),
		expiresAt,
	}
	query, args := buildInsertQuery(fields, args)

	response := &tokenmessage.AddTokenSchemeConnect_Response{}
	if err := h.DB.Conn.QueryRowContext(ctx, query, args...).Scan(&response.Token); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, trace.MsgFailedToAdd), log, nil)
	}
	response.ExpiredAt = timestamppb.New(expiresAt)

	return response, nil
}

// UseTokenSchemeConnect consumes a one-time telnet connection token and
// returns its profile, scheme and active scheme membership.
func (h *Handler) UseTokenSchemeConnect(ctx context.Context, in *tokenmessage.UseTokenSchemeConnect_Request) (*tokenmessage.UseTokenSchemeConnect_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	var profileID, schemeID string
	err := h.DB.Conn.QueryRowContext(ctx, `
    UPDATE "token"
    SET "status" = $2, "updated_at" = NOW()
    WHERE "id" = $1
      AND "section" = $3
      AND "action" = $4
      AND "status" = $5
      AND "expired_at" > NOW()
    RETURNING "profile_id", "scheme_id"
  `,
		in.GetToken(),
		tokenenum.Status_used,
		tokenenum.Section_scheme,
		tokenenum.Action_connect,
		tokenenum.Status_sent,
	).Scan(&profileID, &schemeID)
	if err != nil {
		return nil, trace.Error(status.Error(codes.NotFound, trace.MsgTokenIsInvalid), log, nil)
	}

	schemeMemberID, err := h.activeSchemeMemberID(ctx, profileID, schemeID)
	if err != nil {
		return nil, trace.Error(status.Error(codes.NotFound, trace.MsgTokenIsInvalid), log, nil)
	}

	return &tokenmessage.UseTokenSchemeConnect_Response{
		ProfileId:      profileID,
		SchemeId:       schemeID,
		SchemeMemberId: schemeMemberID,
	}, nil
}

func (h *Handler) activeSchemeMemberID(ctx context.Context, profileID, schemeID string) (string, error) {
	var schemeMemberID string
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT "scheme_member"."id"
    FROM "scheme_member"
      INNER JOIN "project_member" ON "project_member"."id" = "scheme_member"."project_member_id"
      INNER JOIN "scheme" ON "scheme"."id" = "scheme_member"."scheme_id"
    WHERE "project_member"."profile_id" = $1
      AND "scheme_member"."scheme_id" = $2
      AND "scheme_member"."active" = TRUE
      AND "scheme"."active" = TRUE
  `, profileID, schemeID).Scan(&schemeMemberID)
	return schemeMemberID, err
}

func (h *Handler) hasSchemeMembership(ctx context.Context, profileID, schemeID string) (bool, error) {
	var exists bool
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT EXISTS(
      SELECT 1
      FROM "scheme_member"
        INNER JOIN "project_member" ON "project_member"."id" = "scheme_member"."project_member_id"
      WHERE "project_member"."profile_id" = $1
        AND "scheme_member"."scheme_id" = $2
    )
  `, profileID, schemeID).Scan(&exists)
	return exists, err
}

// enrollProjectOwner lets a project owner obtain a connect token without first
// creating an explicit project/scheme membership. The audit subsystem needs a
// scheme_member id, so the owner is enrolled lazily and permanently.
func (h *Handler) enrollProjectOwner(ctx context.Context, profileID, schemeID string) (string, error) {
	tx, err := h.DB.Conn.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var projectID string
	err = tx.QueryRowContext(ctx, `
    SELECT "id"
    FROM "project"
    WHERE "id" = (SELECT "project_id" FROM "scheme" WHERE "id" = $1 AND "active" = TRUE)
      AND "owner_id" = $2
    FOR UPDATE
  `, schemeID, profileID).Scan(&projectID)
	if err != nil {
		return "", err
	}

	var projectMemberID string
	err = tx.QueryRowContext(ctx, `
    INSERT INTO "project_member" ("project_id", "profile_id", "role", "active")
    SELECT $1, $2, 1, TRUE
    WHERE NOT EXISTS (
      SELECT 1 FROM "project_member" WHERE "project_id" = $1 AND "profile_id" = $2
    )
    RETURNING "id"
  `, projectID, profileID).Scan(&projectMemberID)
	if err == sql.ErrNoRows {
		err = tx.QueryRowContext(ctx, `
      SELECT "id" FROM "project_member"
      WHERE "project_id" = $1 AND "profile_id" = $2
      ORDER BY "created_at" ASC LIMIT 1
    `, projectID, profileID).Scan(&projectMemberID)
	}
	if err != nil {
		return "", err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE "project_member" SET "active" = TRUE WHERE "id" = $1`, projectMemberID); err != nil {
		return "", err
	}

	var schemeMemberID string
	err = tx.QueryRowContext(ctx, `
    INSERT INTO "scheme_member" ("scheme_id", "project_member_id", "active")
    SELECT $1, $2, TRUE
    WHERE NOT EXISTS (
      SELECT 1 FROM "scheme_member" WHERE "scheme_id" = $1 AND "project_member_id" = $2
    )
    RETURNING "id"
  `, schemeID, projectMemberID).Scan(&schemeMemberID)
	if err == sql.ErrNoRows {
		err = tx.QueryRowContext(ctx, `
      SELECT "id" FROM "scheme_member"
      WHERE "scheme_id" = $1 AND "project_member_id" = $2
      ORDER BY "created_at" ASC LIMIT 1
    `, schemeID, projectMemberID).Scan(&schemeMemberID)
	}
	if err != nil {
		return "", err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE "scheme_member" SET "active" = TRUE WHERE "id" = $1`, schemeMemberID); err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return schemeMemberID, nil
}
