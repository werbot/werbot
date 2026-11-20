package access

import (
	"context"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/access/dbq"
)

// Store is the persistence the use cases need.
type Store interface {
	CreateBinding(ctx context.Context, arg dbq.CreateBindingParams) (dbq.CreateBindingRow, error)
	DeleteBinding(ctx context.Context, arg dbq.DeleteBindingParams) (dbq.DeleteBindingRow, error)
	ListBindings(ctx context.Context, arg dbq.ListBindingsParams) ([]dbq.ListBindingsRow, error)

	ListApprovalPolicies(ctx context.Context, orgID uuid.UUID) ([]dbq.ListApprovalPoliciesRow, error)
	CreateApprovalPolicy(ctx context.Context, arg dbq.CreateApprovalPolicyParams) (dbq.CreateApprovalPolicyRow, error)
	UpdateApprovalPolicy(ctx context.Context, arg dbq.UpdateApprovalPolicyParams) (dbq.UpdateApprovalPolicyRow, error)
	DeleteApprovalPolicy(ctx context.Context, arg dbq.DeleteApprovalPolicyParams) (dbq.DeleteApprovalPolicyRow, error)
	CountPolicyRefs(ctx context.Context, arg dbq.CountPolicyRefsParams) (dbq.CountPolicyRefsRow, error)
}
