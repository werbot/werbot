package credential

import (
	"context"

	"github.com/werbot/werbot/internal/credential/dbq"
)

// Store is the persistence the use cases need.
type Store interface {
	UpsertCredential(ctx context.Context, arg dbq.UpsertCredentialParams) (int64, error)
	DeleteCredential(ctx context.Context, arg dbq.DeleteCredentialParams) (int64, error)
	GetCredential(ctx context.Context, arg dbq.GetCredentialParams) (dbq.GetCredentialRow, error)
	ListCredentialKinds(ctx context.Context, arg dbq.ListCredentialKindsParams) ([]dbq.ListCredentialKindsRow, error)
}
