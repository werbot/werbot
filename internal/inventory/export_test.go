package inventory

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PickGateway runs pickGateway without an explicit pool in tx.
func PickGateway(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) (*uuid.UUID, error) {
	return pickGateway(ctx, newStore(tx), orgID, nil)
}

// GCSchedule is the gateway.gc schedule.
var GCSchedule = gcSchedule
