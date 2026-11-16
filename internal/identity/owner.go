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
