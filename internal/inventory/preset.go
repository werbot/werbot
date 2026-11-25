package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/inventory/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

var (
	presetNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	presetIconRe = regexp.MustCompile(`^[a-z0-9-]{0,64}$`)
	// presetBaseTypes are the only types a preset may build on.
	presetBaseTypes  = []string{"tcp", "udp"}
	presetCategories = []connector.Category{connector.CategoryServers, connector.CategoryDatabases, connector.CategoryApplications,
		connector.CategoryDesktops, connector.CategoryContainers, connector.CategoryClouds}
)

// Preset names a tcp or udp base type with default ports, a category and a launch hint.
type Preset struct {
	ID, OrgID                                               uuid.UUID
	Name, BaseType, DisplayName, Icon, Category, LaunchHint string
	DefaultPorts                                            []connector.PortRange
	CreatedAt                                               time.Time
}

// PresetInput describes a preset; Name and BaseType never change after creation.
type PresetInput struct {
	Name, BaseType, DisplayName, Icon, Category, LaunchHint string
	DefaultPorts                                            []connector.PortRange
}

func (in PresetInput) validate() error {
	if !presetNameRe.MatchString(in.Name) {
		return invalid("preset name must match %s", presetNameRe)
	}
	if _, builtin := connector.Lookup(in.Name); builtin {
		return invalid("preset name %q is a builtin type", in.Name)
	}
	if !slices.Contains(presetBaseTypes, in.BaseType) {
		return invalid("base_type must be one of %v", presetBaseTypes)
	}
	if n := utf8.RuneCountInString(in.DisplayName); n < 1 || n > 64 {
		return invalid("display_name must be 1 to 64 characters")
	}
	if !presetIconRe.MatchString(in.Icon) {
		return invalid("icon must match %s", presetIconRe)
	}
	if !slices.Contains(presetCategories, connector.Category(in.Category)) {
		return invalid("unknown category %q", in.Category)
	}
	if utf8.RuneCountInString(in.LaunchHint) > 512 {
		return invalid("launch_hint must be at most 512 characters")
	}
	if err := connector.ValidatePorts(in.DefaultPorts); err != nil {
		return invalid("default_ports: %v", err)
	}
	return nil
}

func toPreset(r dbq.ResourcePreset) (Preset, error) {
	p := Preset{ID: r.ID, OrgID: r.OrgID, Name: r.Name, BaseType: r.BaseType, DisplayName: r.DisplayName, Icon: r.Icon,
		Category: r.Category, LaunchHint: r.LaunchHint, CreatedAt: r.CreatedAt}
	if err := json.Unmarshal(r.DefaultPorts, &p.DefaultPorts); err != nil {
		return Preset{}, fmt.Errorf("inventory: decode ports of preset %s: %w", r.ID, err)
	}
	return p, nil
}

// presetErr maps no rows to ErrNotFound, a duplicate name to ErrAlreadyExists
// and a preset still used by a resource (restrict_violation) to ErrFailedPrecondition.
func presetErr(err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("preset not found: %w", apperr.ErrNotFound)
	case isUniqueViolation(err):
		return fmt.Errorf("preset name already used: %w", apperr.ErrAlreadyExists)
	case pgCode(err, "23001"):
		return fmt.Errorf("preset is used by resources: %w", apperr.ErrFailedPrecondition)
	}
	return err
}

func presetAudit(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, action string, id uuid.UUID, data map[string]any) error {
	p, _ := principal.From(ctx)
	return userActor(ctx, p).audit(ctx, tx, orgID, action, "resource_preset", id, data)
}

// ListPresets returns the presets of the organization by name.
func (s *Service) ListPresets(ctx context.Context, orgID uuid.UUID) ([]Preset, error) {
	var out []Preset
	err := s.tx(ctx, orgID, func(st Store, _ pgx.Tx) error {
		rows, err := st.ListResourcePresets(ctx, orgID)
		for _, r := range rows {
			p, cerr := toPreset(r)
			if cerr != nil {
				return cerr
			}
			out = append(out, p)
		}
		return err
	})
	return out, err
}

// GetPreset returns one preset of the organization.
func (s *Service) GetPreset(ctx context.Context, orgID, id uuid.UUID) (Preset, error) {
	var out Preset
	err := s.tx(ctx, orgID, func(st Store, _ pgx.Tx) error {
		r, err := st.GetResourcePreset(ctx, dbq.GetResourcePresetParams{ID: id, OrgID: orgID})
		if err != nil {
			return err
		}
		out, err = toPreset(r)
		return err
	})
	return out, presetErr(err)
}

// CreatePreset adds a preset and audits it in the same transaction.
func (s *Service) CreatePreset(ctx context.Context, orgID uuid.UUID, in PresetInput) (Preset, error) {
	if err := in.validate(); err != nil {
		return Preset{}, err
	}
	ports, err := json.Marshal(in.DefaultPorts)
	if err != nil {
		return Preset{}, err
	}
	p, _ := principal.From(ctx)
	var out Preset
	err = s.tx(ctx, orgID, func(st Store, tx pgx.Tx) error {
		r, err := st.CreateResourcePreset(ctx, dbq.CreateResourcePresetParams{OrgID: orgID, Name: in.Name, BaseType: in.BaseType,
			DisplayName: in.DisplayName, Icon: in.Icon, DefaultPorts: ports, Category: in.Category, LaunchHint: in.LaunchHint, CreatedBy: &p.UserID})
		if err != nil {
			return err
		}
		if out, err = toPreset(r); err != nil {
			return err
		}
		return presetAudit(ctx, tx, orgID, "resource_preset.create", out.ID, map[string]any{"name": in.Name, "base_type": in.BaseType})
	})
	return out, presetErr(err)
}

// UpdatePreset replaces the display fields; a changed Name or BaseType is invalid.
func (s *Service) UpdatePreset(ctx context.Context, orgID, id uuid.UUID, in PresetInput) (Preset, error) {
	if err := in.validate(); err != nil {
		return Preset{}, err
	}
	ports, err := json.Marshal(in.DefaultPorts)
	if err != nil {
		return Preset{}, err
	}
	var out Preset
	err = s.tx(ctx, orgID, func(st Store, tx pgx.Tx) error {
		cur, err := st.GetResourcePreset(ctx, dbq.GetResourcePresetParams{ID: id, OrgID: orgID})
		if err != nil {
			return err
		}
		if cur.Name != in.Name || cur.BaseType != in.BaseType {
			return invalid("name and base_type of a preset never change")
		}
		r, err := st.UpdateResourcePreset(ctx, dbq.UpdateResourcePresetParams{ID: id, OrgID: orgID, DisplayName: in.DisplayName,
			Icon: in.Icon, DefaultPorts: ports, Category: in.Category, LaunchHint: in.LaunchHint})
		if err != nil {
			return err
		}
		if out, err = toPreset(r); err != nil {
			return err
		}
		return presetAudit(ctx, tx, orgID, "resource_preset.update", id, map[string]any{"name": in.Name})
	})
	return out, presetErr(err)
}

// DeletePreset removes a preset no resource uses.
func (s *Service) DeletePreset(ctx context.Context, orgID, id uuid.UUID) error {
	return presetErr(s.tx(ctx, orgID, func(st Store, tx pgx.Tx) error {
		n, err := st.DeleteResourcePreset(ctx, dbq.DeleteResourcePresetParams{ID: id, OrgID: orgID})
		if err != nil {
			return err
		}
		if n == 0 {
			return pgx.ErrNoRows
		}
		return presetAudit(ctx, tx, orgID, "resource_preset.delete", id, nil)
	}))
}
