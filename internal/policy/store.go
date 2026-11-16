package policy

import (
	"context"
	"net/netip"

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
}
