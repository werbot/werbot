package identity

import (
	"context"
	"fmt"
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

// OrgSettings is the org_setting row of one organization.
type OrgSettings struct {
	CLISessionTTL time.Duration // idle lifetime of a CLI session
	CLISessionMax time.Duration // absolute lifetime of a CLI session
}

func settingsFromRow(ttlSecs, maxSecs int64) OrgSettings {
	return OrgSettings{CLISessionTTL: time.Duration(ttlSecs) * time.Second, CLISessionMax: time.Duration(maxSecs) * time.Second}
}

// Settings returns the settings of orgID without a subject, for adapters in
// internal/app. A missing row is an error: there are no code defaults.
func (s *Service) Settings(ctx context.Context, orgID uuid.UUID) (OrgSettings, error) {
	var out OrgSettings
	err := dbx.ReadInOrg(s.pool, orgID, uuid.Nil, func(rd *dbx.Read) error {
		r, err := newStore(rd).GetOrgSettings(ctx, orgID)
		out = settingsFromRow(r.CliSessionTtlSecs, r.CliSessionMaxSecs)
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
		if out, err = applySettingsMask(settingsFromRow(r.CliSessionTtlSecs, r.CliSessionMaxSecs), in, paths); err != nil {
			return err
		}
		if err := validateSettings(out); err != nil {
			return err
		}
		err = st.UpdateOrgSettings(ctx, dbq.UpdateOrgSettingsParams{
			OrgID: p.OrgID, CliSessionTtlSecs: out.CLISessionTTL.Seconds(), CliSessionMaxSecs: out.CLISessionMax.Seconds(),
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
		default:
			return OrgSettings{}, fmt.Errorf("unknown settings field %q: %w", path, apperr.ErrInvalidArgument)
		}
	}
	return cur, nil
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
	}
	return nil
}
