package token

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
)

func getActionWhereClause(action tokenenum.Action) string {
	if action > 0 {
		return fmt.Sprintf(`AND "action" = %d`, action)
	}
	return ""
}

func getStatusWhereClause(status tokenenum.Status) string {
	if status > 0 {
		return fmt.Sprintf(`AND "status" = %d`, status)
	}
	return ""
}

func scanTokens(rows *sql.Rows, limit int32, isAdmin bool) ([]*tokenmessage.Token_Response, error) {
	tokens := make([]*tokenmessage.Token_Response, 0, limit)
	for rows.Next() {
		token := &tokenmessage.Token_Response{}
		var expiredAt, updatedAt, createdAt pgtype.Timestamp
		var profileID, schemeID pgtype.Text

		err := rows.Scan(
			&token.Token,
			&token.Action,
			&token.Status,
			&profileID,
			&schemeID,
			&expiredAt,
			&updatedAt,
			&createdAt,
		)
		if err != nil {
			return nil, err
		}

		token.ProfileId = profileID.String
		token.SchemeId = schemeID.String

		protoutils.SetPgtypeTimestamps(token, map[string]pgtype.Timestamp{
			"expired_at": expiredAt,
			"updated_at": updatedAt,
			"created_at": createdAt,
		})

		if !isAdmin {
			ghoster.Secrets(token, true)
		}

		tokens = append(tokens, token)
	}
	return tokens, nil
}

func buildInsertQuery(fields []string, args []any) (string, []any) {
	sqlFields := postgres.SQLGluingOptions{Separator: ","}.SQLGluing(fields...)
	sqlArgs := make([]string, len(fields))
	for i := range fields {
		sqlArgs[i] = `$` + strconv.Itoa(i+1)
	}
	sqlArgsStr := postgres.SQLGluingOptions{Separator: ","}.SQLGluing(sqlArgs...)

	query := postgres.SQLGluing(`INSERT INTO "token" (`, sqlFields, `) SELECT `, sqlArgsStr, ` RETURNING "id"`)
	return query, args
}

// handleSQLError processes SQL query errors
func handleSQLError(err error) error {
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
	return trace.Error(grpcErr, log, nil)
}

// FindActiveTokenByProfileAndAction searches for active token by profile_id and action
// Returns token ID and error. If token not found, returns empty string and nil
// A token is considered active if:
// - Status is 'sent'
// - Created within the last 24 hours OR expired_at is in the future (if set)
func (h *Handler) FindActiveTokenByProfileAndAction(ctx context.Context, profileID string, action tokenenum.Action) (string, error) {
	var tokenID pgtype.Text
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT "id"
    FROM "token"
    WHERE
      "section" = $1
      AND "action" = $2
      AND "profile_id" = $3
      AND "status" = $4
      AND (
        "created_at" > CURRENT_TIMESTAMP - INTERVAL '24 hour'
        OR ("expired_at" IS NOT NULL AND "expired_at" > CURRENT_TIMESTAMP)
      )
    ORDER BY "created_at" DESC
    LIMIT 1
  `,
		tokenenum.Section_profile,
		action,
		profileID,
		tokenenum.Status_sent,
	).Scan(&tokenID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return tokenID.String, nil
}

// GetOrCreateProfileToken returns existing active token or creates a new one
// Returns token ID, whether it's a new token (first time), and error
func (h *Handler) GetOrCreateProfileToken(ctx context.Context, profileID string, action tokenenum.Action, createToken func(ctx context.Context, profileID string) (string, error)) (tokenID string, isNew bool, err error) {
	existingTokenID, err := h.FindActiveTokenByProfileAndAction(ctx, profileID, action)
	if err != nil {
		return "", false, err
	}
	if existingTokenID != "" {
		return existingTokenID, false, nil
	}
	newTokenID, err := createToken(ctx, profileID)
	if err != nil {
		return "", false, err
	}
	return newTokenID, true, nil
}

// ProfileDataByEmail contains profile data retrieved by email
type ProfileDataByEmail struct {
	ID      string
	Name    string
	Surname string
	Exists  bool
}

// GetProfileDataByEmail retrieves profile data by email
// Returns profile data and error. If profile not found, returns data with Exists=false
func (h *Handler) GetProfileDataByEmail(ctx context.Context, email string) (*ProfileDataByEmail, error) {
	var profileID, name, surname string
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT "id", "name", "surname" FROM "profile" WHERE "email" = $1
  `, email).Scan(&profileID, &name, &surname)
	if err != nil {
		if err == sql.ErrNoRows {
			return &ProfileDataByEmail{Exists: false}, nil
		}
		return nil, err
	}
	return &ProfileDataByEmail{
		ID:      profileID,
		Name:    name,
		Surname: surname,
		Exists:  true,
	}, nil
}

// ValidateTokenStatusAndAction validates token status and action combination
// Returns error if validation fails
func ValidateTokenStatusAndAction(tokenStatus tokenenum.Status, action tokenenum.Action, expectedAction tokenenum.Action, expectedStatus tokenenum.Status) error {
	if action != expectedAction {
		return status.Error(codes.InvalidArgument, "token action mismatch")
	}
	if tokenStatus != expectedStatus {
		return status.Error(codes.InvalidArgument, "token status is invalid for this operation")
	}
	return nil
}

// ValidateTokenForUpdate validates if token can be updated with new status
// Returns error if validation fails
func ValidateTokenForUpdate(isAdmin bool, currentStatus tokenenum.Status, newStatus tokenenum.Status) error {
	if !isAdmin {
		if newStatus == tokenenum.Status_status_unspecified ||
			newStatus == tokenenum.Status_deleted ||
			newStatus == tokenenum.Status_archived {
			return status.Error(codes.NotFound, trace.MsgStatusNotFound)
		}
		if currentStatus == tokenenum.Status_done {
			return status.Error(codes.PermissionDenied, trace.MsgPermissionDenied)
		}
	}
	return nil
}

// CountRecentTokensByProfileAndAction counts tokens created within the last 24 hours
// Used for rate limiting
func (h *Handler) CountRecentTokensByProfileAndAction(ctx context.Context, profileID string, action tokenenum.Action, section tokenenum.Section) (int, error) {
	var count int
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM "token"
    WHERE
      "section" = $1
      AND "action" = $2
      AND "profile_id" = $3
      AND "status" = $4
      AND "created_at" > CURRENT_TIMESTAMP - INTERVAL '24 hour'
  `,
		section,
		action,
		profileID,
		tokenenum.Status_sent,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CheckRateLimit checks if token creation rate limit is exceeded
// maxTokensPer24h: maximum number of tokens allowed per 24 hours
// Returns true if rate limit is exceeded, false otherwise
func (h *Handler) CheckRateLimit(ctx context.Context, profileID string, action tokenenum.Action, section tokenenum.Section, maxTokensPer24h int) (bool, error) {
	count, err := h.CountRecentTokensByProfileAndAction(ctx, profileID, action, section)
	if err != nil {
		return false, err
	}
	return count >= maxTokensPer24h, nil
}

// TokenMetrics contains basic token usage metrics
type TokenMetrics struct {
	Section tokenenum.Section
	Action  tokenenum.Action
	Created int
	Used    int
}

// GetTokenMetrics retrieves basic token usage metrics by section and action
// Returns metrics for token creation and usage counts
func (h *Handler) GetTokenMetrics(ctx context.Context, section tokenenum.Section, action tokenenum.Action) (*TokenMetrics, error) {
	metrics := &TokenMetrics{
		Section: section,
		Action:  action,
	}

	// Count created tokens (status = sent)
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM "token"
    WHERE "section" = $1 AND "action" = $2 AND "status" = $3
  `, section, action, tokenenum.Status_sent).Scan(&metrics.Created)
	if err != nil {
		return nil, err
	}

	// Count used tokens (status = used)
	err = h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM "token"
    WHERE "section" = $1 AND "action" = $2 AND "status" = $3
  `, section, action, tokenenum.Status_used).Scan(&metrics.Used)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}
