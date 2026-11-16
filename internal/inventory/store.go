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

	OrgGatewayIDs(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error)
	CreateGateway(ctx context.Context, arg dbq.CreateGatewayParams) (dbq.CreateGatewayRow, error)
	CreateGatewayIfAbsent(ctx context.Context, arg dbq.CreateGatewayIfAbsentParams) (uuid.UUID, error)
	GetGatewayIDByName(ctx context.Context, arg dbq.GetGatewayIDByNameParams) (uuid.UUID, error)
	AssignUnpooledResources(ctx context.Context, arg dbq.AssignUnpooledResourcesParams) (int64, error)
	ListGateways(ctx context.Context, orgID uuid.UUID) ([]dbq.ListGatewaysRow, error)
	DeleteGateway(ctx context.Context, arg dbq.DeleteGatewayParams) (int64, error)
	GetGatewayHostKey(ctx context.Context, arg dbq.GetGatewayHostKeyParams) (dbq.GetGatewayHostKeyRow, error)

	ListInstances(ctx context.Context, orgID uuid.UUID) ([]dbq.ListInstancesRow, error)
	AuthenticateInstance(ctx context.Context, arg dbq.AuthenticateInstanceParams) (dbq.AuthenticateInstanceRow, error)
	CreateInstance(ctx context.Context, arg dbq.CreateInstanceParams) error
	GetInstanceGatewayID(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	RenewInstanceCert(ctx context.Context, arg dbq.RenewInstanceCertParams) error
	HeartbeatInstance(ctx context.Context, arg dbq.HeartbeatInstanceParams) (int64, error)
	DeleteInstance(ctx context.Context, arg dbq.DeleteInstanceParams) (uuid.UUID, error)
	DeleteExpiredInstances(ctx context.Context) (int64, error)

	CreateJoinToken(ctx context.Context, arg dbq.CreateJoinTokenParams) (dbq.CreateJoinTokenRow, error)
	ListJoinTokens(ctx context.Context, arg dbq.ListJoinTokensParams) ([]dbq.ListJoinTokensRow, error)
	RevokeJoinToken(ctx context.Context, arg dbq.RevokeJoinTokenParams) (uuid.UUID, error)
	SpendJoinToken(ctx context.Context, tokenHash []byte) (dbq.SpendJoinTokenRow, error)
	DeleteExpiredJoinTokens(ctx context.Context) (int64, error)
}
