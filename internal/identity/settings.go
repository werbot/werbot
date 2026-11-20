package identity

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Lock modes: what a gateway does with its sessions while its tunnel is down.
const (
	LockModeBestEffort = "best_effort"
	LockModeStrict     = "strict"
)

var maskOutputModes = []string{"agents", "all", "none"}

// OrgSettings is the org_setting row of one organization.
type OrgSettings struct {
	CLISessionTTL      time.Duration // idle lifetime of a CLI session
	CLISessionMax      time.Duration // absolute lifetime of a CLI session
	GrantDefaultTTL    time.Duration // grant length when a request names none
	GrantMaxTTL        time.Duration // longest grant
	RequestPendingTTL  time.Duration // how long an access request waits for a decision
	CommandApprovalTTL time.Duration // how long a command approval waits for a decision
	LockMode           string        // LockModeBestEffort | LockModeStrict
	MaskOutput         string        // "agents" | "all" | "none": whose session output is masked
}

func settingsFromRow(r dbq.GetOrgSettingsRow) OrgSettings {
	sec := func(n int64) time.Duration { return time.Duration(n) * time.Second }
	return OrgSettings{CLISessionTTL: sec(r.CliSessionTtlSecs), CLISessionMax: sec(r.CliSessionMaxSecs),
		GrantDefaultTTL: sec(r.GrantDefaultTtlSecs), GrantMaxTTL: sec(r.GrantMaxTtlSecs),
		RequestPendingTTL: sec(r.RequestPendingTtlSecs), CommandApprovalTTL: sec(r.CommandApprovalTtlSecs), LockMode: r.LockMode, MaskOutput: r.MaskOutput}
}

// Settings returns the settings of orgID without a subject, for adapters in
// internal/app. A missing row is an error: there are no code defaults.
func (s *Service) Settings(ctx context.Context, orgID uuid.UUID) (OrgSettings, error) {
	var out OrgSettings
	err := dbx.ReadInOrg(s.pool, orgID, uuid.Nil, func(rd *dbx.Read) error {
		r, err := newStore(rd).GetOrgSettings(ctx, orgID)
		out = settingsFromRow(r)
		return err
	})
	if err != nil {
		return OrgSettings{}, fmt.Errorf("read organization settings: %w", err)
	}
	return out, nil
}

// UpdateSettings writes the fields of in named by paths (proto field names)
// and returns the stored settings. Only owner and admin may call it.
func (s *Service) UpdateSettings(ctx context.Context, p principal.Principal, in OrgSettings, paths []string) (OrgSettings, error) {
	if !isManager(p) {
		return OrgSettings{}, apperr.ErrPermissionDenied
	}
	var out OrgSettings
	err := dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		st := newStore(tx)
		r, err := st.LockOrgSettings(ctx, p.OrgID)
		if err != nil {
			return err
		}
		if out, err = applySettingsMask(settingsFromRow(dbq.GetOrgSettingsRow(r)), in, paths); err != nil {
			return err
		}
		if err := validateSettings(out); err != nil {
			return err
		}
		err = st.UpdateOrgSettings(ctx, dbq.UpdateOrgSettingsParams{
			OrgID: p.OrgID, CliSessionTtlSecs: out.CLISessionTTL.Seconds(), CliSessionMaxSecs: out.CLISessionMax.Seconds(),
			GrantDefaultTtlSecs: out.GrantDefaultTTL.Seconds(), GrantMaxTtlSecs: out.GrantMaxTTL.Seconds(),
			RequestPendingTtlSecs: out.RequestPendingTTL.Seconds(), CommandApprovalTtlSecs: out.CommandApprovalTTL.Seconds(),
			LockMode: out.LockMode, MaskOutput: out.MaskOutput,
		})
		if err != nil {
			return err
		}
		return auditlog.Write(ctx, tx, auditlog.Event{
			OrgID: &p.OrgID, ActorUserID: &p.UserID, ActorKind: "user", Action: "settings.update",
			IP: clientip.From(ctx), Data: map[string]any{"fields": paths},
		})
	})
	return out, err
}

// applySettingsMask copies the fields named by paths from in onto cur. An
// empty list or an unknown path is ErrInvalidArgument.
func applySettingsMask(cur, in OrgSettings, paths []string) (OrgSettings, error) {
	if len(paths) == 0 {
		return OrgSettings{}, fmt.Errorf("update_mask is empty: %w", apperr.ErrInvalidArgument)
	}
	for _, path := range paths {
		switch path {
		case "cli_session_ttl":
			cur.CLISessionTTL = in.CLISessionTTL
		case "cli_session_max":
			cur.CLISessionMax = in.CLISessionMax
		case "grant_default_ttl":
			cur.GrantDefaultTTL = in.GrantDefaultTTL
		case "grant_max_ttl":
			cur.GrantMaxTTL = in.GrantMaxTTL
		case "request_pending_ttl":
			cur.RequestPendingTTL = in.RequestPendingTTL
		case "command_approval_ttl":
			cur.CommandApprovalTTL = in.CommandApprovalTTL
		default:
			if !applyMode(&cur, in, path) {
				return OrgSettings{}, fmt.Errorf("unknown settings field %q: %w", path, apperr.ErrInvalidArgument)
			}
		}
	}
	return cur, nil
}

// applyMode copies the enum field named by path and reports whether path is one.
func applyMode(cur *OrgSettings, in OrgSettings, path string) bool {
	switch path {
	case "lock_mode":
		cur.LockMode = in.LockMode
	case "mask_output":
		cur.MaskOutput = in.MaskOutput
	default:
		return false
	}
	return true
}

// validateSettings checks the whole struct; the same bounds are in proto
// rules and in the org_setting CHECK constraints.
func validateSettings(o OrgSettings) error {
	switch {
	case o.CLISessionTTL < time.Hour || o.CLISessionTTL > 720*time.Hour:
		return fmt.Errorf("cli_session_ttl must be between 1h and 720h: %w", apperr.ErrInvalidArgument)
	case o.CLISessionMax < 24*time.Hour || o.CLISessionMax > 2160*time.Hour:
		return fmt.Errorf("cli_session_max must be between 24h and 2160h: %w", apperr.ErrInvalidArgument)
	case o.CLISessionTTL > o.CLISessionMax:
		return fmt.Errorf("cli_session_ttl must not exceed cli_session_max: %w", apperr.ErrInvalidArgument)
	case o.GrantMaxTTL < time.Minute || o.GrantMaxTTL > 14*24*time.Hour:
		return fmt.Errorf("grant_max_ttl must be between 1m and 336h: %w", apperr.ErrInvalidArgument)
	case o.GrantDefaultTTL <= 0 || o.GrantDefaultTTL > o.GrantMaxTTL:
		return fmt.Errorf("grant_default_ttl must be above zero and not exceed grant_max_ttl: %w", apperr.ErrInvalidArgument)
	case o.RequestPendingTTL < time.Minute || o.RequestPendingTTL > 24*time.Hour:
		return fmt.Errorf("request_pending_ttl must be between 1m and 24h: %w", apperr.ErrInvalidArgument)
	case o.CommandApprovalTTL < 30*time.Second || o.CommandApprovalTTL > time.Hour:
		return fmt.Errorf("command_approval_ttl must be between 30s and 1h: %w", apperr.ErrInvalidArgument)
	}
	return validateModes(o)
}

func validateModes(o OrgSettings) error {
	switch {
	case o.LockMode != LockModeBestEffort && o.LockMode != LockModeStrict:
		return fmt.Errorf("lock_mode must be best_effort or strict: %w", apperr.ErrInvalidArgument)
	case !slices.Contains(maskOutputModes, o.MaskOutput):
		return fmt.Errorf("mask_output must be agents, all or none: %w", apperr.ErrInvalidArgument)
	}
	return nil
}
