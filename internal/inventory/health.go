package inventory

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/inventory/dbq"
)

// Health states of a resource.
const (
	HealthUnknown   = "unknown"
	HealthHealthy   = "healthy"
	HealthUnhealthy = "unhealthy"
)

// maxHealthError bounds the stored probe error; the gateway is not trusted with unbounded text.
const maxHealthError = 512

// Health is one probe result of a resource from one gateway instance.
type Health struct {
	ResourceID uuid.UUID
	OK         bool
	Error      string
	CheckedAt  time.Time
}

// HealthStatus is the state of a resource over the online instances of its pool:
// healthy when a probe within 3 min succeeded, unhealthy when all such probes failed.
type HealthStatus struct {
	State     string
	CheckedAt *time.Time // nil while unknown
	Error     string     // of the latest failed probe; empty unless unhealthy
}

func healthStatus(state string, checkedAt time.Time, errText string) HealthStatus {
	h := HealthStatus{State: state}
	if state != HealthUnknown {
		h.CheckedAt = &checkedAt
		if state == HealthUnhealthy {
			h.Error = errText
		}
	}
	return h
}

// ReportHealth upserts the probe results of instanceID. Resources outside the
// instance's pool or the organization are skipped; of repeated ids the newest wins.
func (s *Service) ReportHealth(ctx context.Context, orgID, instanceID uuid.UUID, hs []Health) error {
	if len(hs) == 0 {
		return nil
	}
	arg := dbq.UpsertResourceHealthParams{OrgID: orgID, InstanceID: instanceID}
	for _, h := range hs {
		arg.ResourceIds = append(arg.ResourceIds, h.ResourceID)
		arg.Oks = append(arg.Oks, h.OK)
		arg.Errors = append(arg.Errors, truncate(h.Error, maxHealthError))
		arg.CheckedAts = append(arg.CheckedAts, h.CheckedAt)
	}
	return s.tx(ctx, orgID, func(st Store, _ pgx.Tx) error { return st.UpsertResourceHealth(ctx, arg) })
}

// truncate cuts s to n bytes and drops any rune cut in half.
func truncate(s string, n int) string {
	return strings.ToValidUTF8(s[:min(n, len(s))], "")
}
