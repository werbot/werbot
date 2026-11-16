package access

import (
	"context"

	"github.com/werbot/werbot/internal/access/dbq"
)

// Store is the persistence the use cases need.
type Store interface {
	CreateBinding(ctx context.Context, arg dbq.CreateBindingParams) (dbq.AccessBinding, error)
	DeleteBinding(ctx context.Context, arg dbq.DeleteBindingParams) (int64, error)
	ListBindings(ctx context.Context, arg dbq.ListBindingsParams) ([]dbq.AccessBinding, error)
}
