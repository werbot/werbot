package token

import (
	"context"
	"database/sql"
	"errors"

	profileenum "github.com/werbot/werbot/internal/core/profile/proto/enum"
)

// addProjectMemberForToken grants project membership to the profile accepting
// a project invite. A single INSERT ... SELECT keeps the duplicate check and
// the write atomic, so accepting the same token twice adds no second row.
func addProjectMemberForToken(ctx context.Context, tx *sql.Tx, projectID, profileID string) error {
	_, err := tx.ExecContext(ctx, `
    INSERT INTO "project_member" ("project_id", "profile_id", "role", "active")
    SELECT $1, $2, $3, true
    WHERE NOT EXISTS (
      SELECT 1
      FROM "project_member"
      WHERE
        "project_id" = $1
        AND "profile_id" = $2
        AND "active" = true
    )
  `,
		projectID,
		profileID,
		profileenum.Role_user,
	)
	return err
}

// addSchemeMemberForToken grants scheme access to the profile accepting a
// scheme invite. The profile needs a project_member row first because
// scheme_member references it, so the row is created when missing.
func addSchemeMemberForToken(ctx context.Context, tx *sql.Tx, schemeID, profileID string) error {
	projectMemberID, err := schemeProjectMemberID(ctx, tx, schemeID, profileID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
    INSERT INTO "scheme_member" ("scheme_id", "project_member_id", "active")
    SELECT $1, $2, true
    WHERE NOT EXISTS (
      SELECT 1
      FROM "scheme_member"
      WHERE "scheme_id" = $1 AND "project_member_id" = $2
    )
  `,
		schemeID,
		projectMemberID,
	)
	return err
}

// schemeProjectMemberID returns the project_member id for the scheme's project,
// inserting the membership row when the profile does not have one yet.
func schemeProjectMemberID(ctx context.Context, tx *sql.Tx, schemeID, profileID string) (string, error) {
	const selectMember = `
    SELECT "project_member"."id"
    FROM "project_member"
    INNER JOIN "scheme" ON "scheme"."project_id" = "project_member"."project_id"
    WHERE "scheme"."id" = $1 AND "project_member"."profile_id" = $2
    ORDER BY "project_member"."active" DESC, "project_member"."created_at" ASC
    LIMIT 1
  `

	var memberID string
	err := tx.QueryRowContext(ctx, selectMember, schemeID, profileID).Scan(&memberID)
	if err == nil {
		return memberID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	err = tx.QueryRowContext(ctx, `
    INSERT INTO "project_member" ("project_id", "profile_id", "role", "active")
    SELECT "scheme"."project_id", $2, $3, true
    FROM "scheme"
    WHERE
      "scheme"."id" = $1
      AND NOT EXISTS (
        SELECT 1
        FROM "project_member"
        WHERE
          "project_id" = "scheme"."project_id"
          AND "profile_id" = $2
      )
    RETURNING "id"
  `,
		schemeID,
		profileID,
		profileenum.Role_user,
	).Scan(&memberID)
	if errors.Is(err, sql.ErrNoRows) {
		// A concurrent accept inserted the row between the SELECT and INSERT.
		err = tx.QueryRowContext(ctx, selectMember, schemeID, profileID).Scan(&memberID)
	}
	return memberID, err
}
