package token

import (
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

// handleSQLError обрабатывает ошибки SQL-запросов
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
