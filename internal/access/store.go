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

	ListMembers(ctx context.Context, orgID uuid.UUID) ([]dbq.ListMembersRow, error)
	ListTargets(ctx context.Context, orgID uuid.UUID) ([]dbq.ListTargetsRow, error)
	CreateApproval(ctx context.Context, arg dbq.CreateApprovalParams) (dbq.Approval, error)
	GetApproval(ctx context.Context, arg dbq.GetApprovalParams) (dbq.Approval, error)
	GetApprovalState(ctx context.Context, arg dbq.GetApprovalStateParams) (dbq.GetApprovalStateRow, error)
	DecideApproval(ctx context.Context, arg dbq.DecideApprovalParams) (dbq.Approval, error)
	CancelApproval(ctx context.Context, arg dbq.CancelApprovalParams) (uuid.UUID, error)
	CreateGrants(ctx context.Context, arg dbq.CreateGrantsParams) ([]dbq.CreateGrantsRow, error)
	NotifyApprovalDecided(ctx context.Context, payload string) error
	ListApprovals(ctx context.Context, arg dbq.ListApprovalsParams) ([]dbq.ListApprovalsRow, error)
	ExpireApprovals(ctx context.Context) ([]dbq.ExpireApprovalsRow, error)
	DecidedCommands(ctx context.Context, arg dbq.DecidedCommandsParams) ([]dbq.Approval, error)

	ActiveGrants(ctx context.Context, orgID uuid.UUID) ([]dbq.ActiveGrantsRow, error)
	ListGrants(ctx context.Context, arg dbq.ListGrantsParams) ([]dbq.ListGrantsRow, error)
	GrantForUpdate(ctx context.Context, arg dbq.GrantForUpdateParams) (dbq.GrantForUpdateRow, error)
	RevokeGrant(ctx context.Context, arg dbq.RevokeGrantParams) error
	ReviewGrant(ctx context.Context, arg dbq.ReviewGrantParams) error

	CreateLock(ctx context.Context, arg dbq.CreateLockParams) (dbq.Lock, error)
	DeleteLock(ctx context.Context, arg dbq.DeleteLockParams) (dbq.DeleteLockRow, error)
	ActiveLocks(ctx context.Context, orgID uuid.UUID) ([]dbq.Lock, error)
	ListLocks(ctx context.Context, orgID uuid.UUID) ([]dbq.Lock, error)
}
