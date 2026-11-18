package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
)

// lookupRef runs a gateway lookup in orgID; no member row is ErrNotFound.
// One statement returns the member with its teams.
func (s *Service) lookupRef(ctx context.Context, orgID uuid.UUID, q func(Store) (dbq.LookupLoginUserRow, error)) (UserRef, error) {
	var ref UserRef
	err := dbx.InOrg(ctx, s.pool, orgID, uuid.Nil, func(tx pgx.Tx) error {
		r, err := q(newStore(tx))
		ref = UserRef{UserID: r.ID, Login: r.Login, Role: r.Role, TeamIDs: r.TeamIds}
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return UserRef{}, fmt.Errorf("user not found: %w", apperr.ErrNotFound)
	}
	return ref, err
}

// LookupSSHKey returns the member of orgID who owns the key with fingerprintSHA256.
func (s *Service) LookupSSHKey(ctx context.Context, orgID uuid.UUID, fingerprintSHA256 string) (UserRef, error) {
	return s.lookupRef(ctx, orgID, func(st Store) (dbq.LookupLoginUserRow, error) {
		r, err := st.LookupSSHKeyUser(ctx, dbq.LookupSSHKeyUserParams{OrgID: orgID, FingerprintSha256: fingerprintSHA256})
		return dbq.LookupLoginUserRow(r), err
	})
}

// LookupLogin returns the member of orgID with the given login.
func (s *Service) LookupLogin(ctx context.Context, orgID uuid.UUID, login string) (UserRef, error) {
	return s.lookupRef(ctx, orgID, func(st Store) (dbq.LookupLoginUserRow, error) {
		return st.LookupLoginUser(ctx, dbq.LookupLoginUserParams{OrgID: orgID, Login: login})
	})
}
