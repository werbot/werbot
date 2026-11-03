package audit

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal/trace"
)

// auditAccess reports whether the audit exists and whether the profile owns the
// project or is attached to the audit's scheme member. The check runs in one
// query and never loads the audit row into Go.
func (h *Handler) auditAccess(ctx context.Context, auditID, profileID string) (bool, bool, error) {
	var exists, allowed bool
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      EXISTS (
        SELECT 1
        FROM "audit"
        WHERE "id" = $1
      ),
      EXISTS (
        SELECT 1
        FROM "audit" a
        INNER JOIN "scheme_member" sm ON sm."id" = a."member_id"
        INNER JOIN "scheme" s ON s."id" = sm."scheme_id"
        INNER JOIN "project" p ON p."id" = s."project_id"
        INNER JOIN "project_member" pm ON pm."id" = sm."project_member_id"
        WHERE
          a."id" = $1
          AND (p."owner_id" = $2 OR pm."profile_id" = $2)
      )
  `, auditID, profileID).Scan(&exists, &allowed)
	if err != nil {
		return false, false, trace.Error(err, log, nil)
	}

	return exists, allowed, nil
}

// memberAccess reports whether the scheme member exists and whether the profile
// owns its project or is attached to it.
func (h *Handler) memberAccess(ctx context.Context, memberID, profileID string) (bool, bool, error) {
	var exists, allowed bool
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      EXISTS (
        SELECT 1
        FROM "scheme_member"
        WHERE "id" = $1
      ),
      EXISTS (
        SELECT 1
        FROM "scheme_member" sm
        INNER JOIN "scheme" s ON s."id" = sm."scheme_id"
        INNER JOIN "project" p ON p."id" = s."project_id"
        INNER JOIN "project_member" pm ON pm."id" = sm."project_member_id"
        WHERE
          sm."id" = $1
          AND (p."owner_id" = $2 OR pm."profile_id" = $2)
      )
  `, memberID, profileID).Scan(&exists, &allowed)
	if err != nil {
		return false, false, trace.Error(err, log, nil)
	}

	return exists, allowed, nil
}

func permissionDenied() error {
	return trace.Error(status.Error(codes.PermissionDenied, trace.MsgPermissionDenied), log, nil)
}

func auditNotFound() error {
	return trace.Error(status.Error(codes.NotFound, trace.MsgAuditNotFound), log, nil)
}

func memberNotFound() error {
	return trace.Error(status.Error(codes.NotFound, trace.MsgMemberNotFound), log, nil)
}
