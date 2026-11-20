package policy

import (
	"context"
	"net/netip"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/werbot/werbot/internal/policy/dbq"
)

// Store is the persistence the use cases need.
type Store interface {
	ListNetworkRules(ctx context.Context, arg dbq.ListNetworkRulesParams) ([]dbq.ListNetworkRulesRow, error)
	ListCountryRules(ctx context.Context, arg dbq.ListCountryRulesParams) ([]dbq.ListCountryRulesRow, error)
	ListSchedule(ctx context.Context, arg dbq.ListScheduleParams) ([]dbq.ListScheduleRow, error)
	DeleteNetworkRules(ctx context.Context, arg dbq.DeleteNetworkRulesParams) error
	DeleteCountryRules(ctx context.Context, arg dbq.DeleteCountryRulesParams) error
	DeleteSchedule(ctx context.Context, arg dbq.DeleteScheduleParams) error
	InsertNetworkRule(ctx context.Context, arg dbq.InsertNetworkRuleParams) error
	InsertCountryRule(ctx context.Context, arg dbq.InsertCountryRuleParams) error
	InsertSlot(ctx context.Context, arg dbq.InsertSlotParams) error
	IPBlocked(ctx context.Context, ip netip.Addr) (bool, error)
	ListBlocklist(ctx context.Context) ([]netip.Prefix, error)
	GetGeoIPData(ctx context.Context) ([]byte, error)
	DeleteBlocklistSource(ctx context.Context, source string) error
	ReplaceBlocklist(ctx context.Context, arg []dbq.ReplaceBlocklistParams) (int64, error)
	GetGeoIPMonth(ctx context.Context) (pgtype.Date, error)
	UpsertGeoIP(ctx context.Context, arg dbq.UpsertGeoIPParams) error
	NotifyGeoIPChanged(ctx context.Context) error
	ListCommandRules(ctx context.Context, orgID uuid.UUID) ([]dbq.CommandRule, error)
	CreateCommandRule(ctx context.Context, arg dbq.CreateCommandRuleParams) (dbq.CommandRule, error)
	UpdateCommandRule(ctx context.Context, arg dbq.UpdateCommandRuleParams) (dbq.CommandRule, error)
	DeleteCommandRule(ctx context.Context, arg dbq.DeleteCommandRuleParams) (dbq.CommandRule, error)
	CountCommandRuleRefs(ctx context.Context, arg dbq.CountCommandRuleRefsParams) (dbq.CountCommandRuleRefsRow, error)
	ListClassRules(ctx context.Context, orgID uuid.UUID) ([]dbq.CommandClassRule, error)
	CreateClassRule(ctx context.Context, arg dbq.CreateClassRuleParams) (dbq.CommandClassRule, error)
	DeleteClassRule(ctx context.Context, arg dbq.DeleteClassRuleParams) (dbq.CommandClassRule, error)
	ListCustomPolicies(ctx context.Context, orgID uuid.UUID) ([]dbq.CustomPolicy, error)
	CreateCustomPolicy(ctx context.Context, arg dbq.CreateCustomPolicyParams) (dbq.CustomPolicy, error)
	UpdateCustomPolicy(ctx context.Context, arg dbq.UpdateCustomPolicyParams) (dbq.CustomPolicy, error)
	DeleteCustomPolicy(ctx context.Context, arg dbq.DeleteCustomPolicyParams) (dbq.CustomPolicy, error)
	IsMember(ctx context.Context, arg dbq.IsMemberParams) (bool, error)
}
