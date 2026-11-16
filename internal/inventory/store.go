package inventory

import (
	"context"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/inventory/dbq"
)

// Store is the persistence the use cases need.
type Store interface {
	CreateResource(ctx context.Context, arg dbq.CreateResourceParams) (uuid.UUID, error)
	UpdateResource(ctx context.Context, arg dbq.UpdateResourceParams) (int64, error)
	DeleteResource(ctx context.Context, arg dbq.DeleteResourceParams) (int64, error)
	GetResource(ctx context.Context, arg dbq.GetResourceParams) (dbq.GetResourceRow, error)
	ListResources(ctx context.Context, arg dbq.ListResourcesParams) ([]dbq.ListResourcesRow, error)
	DeleteLabels(ctx context.Context, arg dbq.DeleteLabelsParams) error
	InsertLabels(ctx context.Context, arg dbq.InsertLabelsParams) error
	InsertHostKey(ctx context.Context, arg dbq.InsertHostKeyParams) (int64, error)
	GetHostKeyFingerprint(ctx context.Context, arg dbq.GetHostKeyFingerprintParams) (string, error)
	DeleteHostKey(ctx context.Context, arg dbq.DeleteHostKeyParams) (int64, error)
}
