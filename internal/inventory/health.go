package inventory

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/inventory/dbq"
	"github.com/werbot/werbot/internal/platform/auditlog"
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
	// AgentReadonlyStatus is a check of the agent_readonly credential (ok,
	// has_write or error; empty: none in this result) of check version
	// AgentReadonlyVersion, with its findings.
	AgentReadonlyStatus, AgentReadonlyDetail string
	AgentReadonlyVersion                     int64
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
	return s.tx(ctx, orgID, func(st Store, tx pgx.Tx) error {
		if err := st.UpsertResourceHealth(ctx, arg); err != nil {
			return err
		}
		return setAgentReadonly(ctx, st, tx, orgID, instanceID, hs)
	})
}

// setAgentReadonly stores the agent_readonly checks of hs that match the
// current check version and audits each change of status or detail.
func setAgentReadonly(ctx context.Context, st Store, tx pgx.Tx, orgID, instanceID uuid.UUID, hs []Health) error {
	arg := dbq.SetAgentReadonlyCheckedParams{OrgID: orgID, InstanceID: instanceID}
	for _, h := range hs {
		if h.AgentReadonlyStatus == "" {
			continue
		}
		arg.ResourceIds = append(arg.ResourceIds, h.ResourceID)
		arg.Statuses = append(arg.Statuses, h.AgentReadonlyStatus)
		arg.Details = append(arg.Details, truncate(h.AgentReadonlyDetail, maxHealthError))
		arg.Versions = append(arg.Versions, h.AgentReadonlyVersion)
		arg.CheckedAts = append(arg.CheckedAts, h.CheckedAt)
	}
	if len(arg.ResourceIds) == 0 {
		return nil
	}
	rows, err := st.SetAgentReadonlyChecked(ctx, arg)
	if err != nil {
		return err
	}
	for _, r := range rows {
		e := auditlog.Event{OrgID: &orgID, ActorKind: "gateway", Action: "resource.agent_readonly.check", TargetType: "resource",
			TargetID: r.ID.String(), Data: map[string]any{"status": r.AgentReadonlyStatus, "detail": r.AgentReadonlyDetail}}
		if err := auditlog.Write(ctx, tx, e); err != nil {
			return err
		}
	}
	return nil
}

// truncate cuts s to n bytes and drops any rune cut in half.
func truncate(s string, n int) string {
	return strings.ToValidUTF8(s[:min(n, len(s))], "")
}
