package identity

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// ensureNotLastOwner fails with ErrFailedPrecondition when userID is the only
// owner of orgID. It locks the owner rows until the transaction ends, so two
// concurrent changes cannot both remove the last owners.
func ensureNotLastOwner(ctx context.Context, st Store, orgID, userID uuid.UUID) error {
	owners, err := st.LockOrgOwners(ctx, orgID)
	if err != nil {
		return err
	}
	if len(owners) == 1 && slices.Contains(owners, userID) {
		return fmt.Errorf("last owner: %w", apperr.ErrFailedPrecondition)
	}
	return nil
}

// ensureNotSoleOwner fails with ErrFailedPrecondition when userID is the only
// owner of any organization, so deleting the user cannot orphan one. It locks
// the owner rows of those organizations until the transaction ends.
func ensureNotSoleOwner(ctx context.Context, st Store, userID uuid.UUID) error {
	rows, err := st.LockUserOwnedOrgs(ctx, userID)
	if err != nil {
		return err
	}
	owners := map[uuid.UUID][]uuid.UUID{}
	for _, r := range rows {
		owners[r.OrgID] = append(owners[r.OrgID], r.UserID)
	}
	for _, ids := range owners {
		if len(ids) == 1 && ids[0] == userID {
			return fmt.Errorf("sole owner of an organization: %w", apperr.ErrFailedPrecondition)
		}
	}
	return nil
}
