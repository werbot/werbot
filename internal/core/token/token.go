package token

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// Token retrieves token details by token ID
// Returns token information including section, action, status, and associated metadata
// Validates token expiration if expired_at is set
// Masks sensitive data (owner_id, profile_id, project_id, scheme_id) for non-admin users
func (h *Handler) Token(ctx context.Context, in *tokenmessage.Token_Request) (*tokenmessage.Token_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &tokenmessage.Token_Response{}
	var expiredAt, updatedAt, createdAt pgtype.Timestamp
	var ownerID, profileID, projectID, schemeID pgtype.Text
	var jsonb []byte

	query := `
    SELECT
      "section",
      "action",
      "status",
      "profile_id",
      "project_id",
      "scheme_id",
      "data",
      "expired_at",
      "updated_at",
      "created_at",
      CASE
        WHEN "section" = 1 THEN "profile_id"
        WHEN "section" IN (2, 4) THEN (
          SELECT "owner_id" FROM "project" WHERE "id" = "token"."project_id"
        )
        WHEN "section" = 3 THEN (
          SELECT "project"."owner_id" FROM "project" LEFT JOIN "scheme" ON "project"."id"="scheme"."project_id" WHERE "scheme"."id"="token"."scheme_id"
        )
        ELSE NULL
      END AS "owner_id"
    FROM "token"
    WHERE "id" = $1
  `

	err := h.DB.Conn.QueryRowContext(ctx, query, in.GetToken()).Scan(
		&response.Section,
		&response.Action,
		&response.Status,
		&profileID,
		&projectID,
		&schemeID,
		&jsonb,
		&expiredAt,
		&updatedAt,
		&createdAt,
		&ownerID,
	)
	if err != nil {
		return nil, trace.Error(status.Error(codes.NotFound, trace.MsgTokenNotFound), log, nil)
	}

	// Validate token expiration: check expired_at if set, otherwise use 24-hour default
	if expiredAt.Valid {
		now := time.Now()
		if expiredAt.Time.Before(now) {
			return nil, trace.Error(status.Error(codes.InvalidArgument, "token has expired"), log, nil)
		}
	}

	response.OwnerId = ownerID.String
	response.ProfileId = profileID.String
	response.ProjectId = projectID.String
	response.SchemeId = schemeID.String

	switch response.GetSection() {
	// profile section:
	case tokenenum.Section_profile, tokenenum.Section_project:
		data := &tokenmessage.MetaDataProfile{}
		if err := protojson.Unmarshal(jsonb, data); err != nil {
			return nil, trace.Error(err, log, nil)
		}
		response.Data = &tokenmessage.Token_Response_Profile{Profile: data}

	// scheme section:
	case tokenenum.Section_scheme:
		data := &tokenmessage.MetaDataScheme{}
		if err := protojson.Unmarshal(jsonb, data); err != nil {
			return nil, trace.Error(err, log, nil)
		}
		response.Data = &tokenmessage.Token_Response_Scheme{Scheme: data}

	// agent section:
	case tokenenum.Section_agent:
		data := &tokenmessage.MetaDataAgent{}
		if err := protojson.Unmarshal(jsonb, data); err != nil {
			return nil, trace.Error(err, log, nil)
		}
		response.Data = &tokenmessage.Token_Response_Agent{Agent: data}
	}

	protoutils.SetPgtypeTimestamps(response, map[string]pgtype.Timestamp{
		"expired_at": expiredAt,
		"updated_at": updatedAt,
		"created_at": createdAt,
	})

	if !in.GetIsAdmin() {
		ghoster.Secrets(response, true)
	}

	return response, nil
}

/*
// UpdateToken is ...
func (h *Handler) UpdateToken(ctx context.Context, in *tokenmessage.UpdateToken_Request) (*tokenmessage.UpdateToken_Response, error) {
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

	if tokenData.GetSection() == tokenenum.Section_project || tokenData.GetSection() == tokenenum.Section_scheme {
		ownerId := in.GetOwnerId()
		if ownerId == "" {
			return nil, trace.Error(status.Error(codes.InvalidArgument, "owner_id: value is required"), log, nil)
		}
		// Check ownership rights for the one who wants to change the record
		if ownerId != tokenData.GetOwnerId() {
			return nil, trace.Error(status.Error(codes.NotFound, trace.MsgOwnerNotFound), log, nil)
		}
	}

	// SQL constructor
	sql := `UPDATE "token" SET "status" = $2`
	args := []any{in.GetToken(), in.GetStatus()}
	phCount := 3 // Start from $3 since $1 and $2 are already used

	// helper function to add conditions to the SQL statement
	addCondition := func(fieldName, value string) error {
		if value == "" {
			return trace.Error(status.Error(codes.InvalidArgument, fmt.Sprintf("%s: value is required", fieldName)), log, nil)
		}
		sql += fmt.Sprintf(`, "%s" = $%d`, fieldName, phCount)
		args = append(args, value)
		phCount++
		return nil
	}

	// section-action-status: description
	tokenAction := tokenData.GetAction()
	switch tokenData.GetSection() {
	// profile section:
	case tokenenum.Section_profile:
		// 1-2-1: registration of a new profile and sending a notification by email
		if tokenAction == tokenenum.Action_register && in.GetStatus() == tokenenum.Status_done {
			if err := addCondition("profile_id", in.GetProfileId()); err != nil {
				return nil, err
			}
		}

	// project section:
	case tokenenum.Section_project:
		// 2-3-1: adding a new user to the project and sending a notification by email
		if tokenAction == tokenenum.Action_add && in.GetStatus() == tokenenum.Status_done {
			if err := addCondition("profile_id", in.GetProfileId()); err != nil {
				return nil, err
			}
		}

	// scheme section:
	case tokenenum.Section_scheme:
		// 3-3-1: adding schema settings by link and sending a notification by email
		if tokenAction == tokenenum.Action_add && in.GetStatus() == tokenenum.Status_sent {
			addCondition("profile_id", in.GetProfileId()) // optional
			if err := addCondition("scheme_id", in.GetSchemeId()); err != nil {
				return nil, err
			}
		}
		// 3-8-1: share access for the user, send an invitation by email
		if tokenAction == tokenenum.Action_access && in.GetStatus() == tokenenum.Status_sent {
			if err := addCondition("profile_id", in.GetProfileId()); err != nil {
				return nil, err
			}
		}

	// agent section:
	case tokenenum.Section_agent:
		// 4-3-1: adding a new server through the agent
		if tokenAction == tokenenum.Action_add && in.GetStatus() == tokenenum.Status_done {
			if err := addCondition("scheme_id", in.GetSchemeId()); err != nil {
				return nil, err
			}
		}
	}

	sql += ` WHERE "id" = $1`

	result, err := h.DB.Conn.ExecContext(ctx, sql, args...)
	// human-readable errors
	if err != nil {
		var grpcErr error
		if pgErr, ok := err.(*pgconn.PgError); ok {
			constraintErrors := map[string]string{
				"token_profile_id_fkey": trace.MsgProfileNotFound,
				"token_project_id_fkey": trace.MsgProjectNotFound,
				"token_scheme_id_fkey":  trace.MsgSchemeNotFound,
			}
			if msg, exists := constraintErrors[pgErr.ConstraintName]; exists {
				grpcErr = status.Error(codes.NotFound, msg)
			}
		} else {
			grpcErr = status.Error(codes.InvalidArgument, trace.MsgFailedToUpdate)
		}

		return nil, trace.Error(grpcErr, log, nil)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		return nil, trace.Error(status.Error(codes.NotFound, trace.MsgTokenNotFound), log, nil)
	}

	// TODO: send email message if need
	// use data from tokenData

	return &tokenmessage.UpdateToken_Response{}, nil
}
*/

// DeleteToken marks a token as deleted by setting status to deleted
// Requires owner_id for non-admin users to verify ownership
// Returns error if token not found or ownership verification fails
func (h *Handler) DeleteToken(ctx context.Context, in *tokenmessage.DeleteToken_Request) (*tokenmessage.DeleteToken_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	if !in.GetIsAdmin() && in.GetOwnerId() == "" {
		return nil, trace.Error(status.Error(codes.InvalidArgument, trace.MsgInvalidArgument), log, "owner_id: value is required")
	}

	// get core data for token
	tokenData, err := h.Token(ctx, &tokenmessage.Token_Request{
		IsAdmin: true,
		Token:   in.GetToken(),
	})
	if err != nil || tokenData.GetOwnerId() != in.GetOwnerId() {
		return nil, trace.Error(status.Error(codes.NotFound, trace.MsgTokenNotFound), log, nil)
	}

	result, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "token"
    SET "status" = $2
    WHERE "id" = $1
  `,
		in.GetToken(),
		tokenenum.Status_deleted,
	)
	if err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, trace.MsgFailedToDelete), log, nil)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		return nil, trace.Error(status.Error(codes.NotFound, trace.MsgTokenNotFound), log, nil)
	}

	return &tokenmessage.DeleteToken_Response{}, nil
}

// ArchivedToken marks a token as archived by setting status to archived
// Requires owner_id for non-admin users to verify ownership
// Returns error if token not found or ownership verification fails
func (h *Handler) ArchivedToken(ctx context.Context, in *tokenmessage.ArchivedToken_Request) (*tokenmessage.ArchivedToken_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	if !in.GetIsAdmin() && in.GetOwnerId() == "" {
		return nil, trace.Error(status.Error(codes.InvalidArgument, trace.MsgInvalidArgument), log, "owner_id: value is required")
	}

	// get core data for token
	tokenData, err := h.Token(ctx, &tokenmessage.Token_Request{
		IsAdmin: true,
		Token:   in.GetToken(),
	})
	if err != nil || tokenData.GetOwnerId() != in.GetOwnerId() {
		return nil, trace.Error(status.Error(codes.NotFound, trace.MsgTokenNotFound), log, nil)
	}

	result, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "token"
    SET "status" = $2
    WHERE "id" = $1
  `,
		in.GetToken(),
		tokenenum.Status_archived,
	)
	if err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, trace.MsgFailedToDelete), log, nil)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		return nil, trace.Error(status.Error(codes.NotFound, trace.MsgTokenNotFound), log, nil)
	}

	return &tokenmessage.ArchivedToken_Response{}, nil
}
