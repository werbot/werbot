package inventory

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	connectorv1 "github.com/werbot/werbot/gen/werbot/connector/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// PresetHandler serves ResourcePresetService; owners and admins only.
type PresetHandler struct {
	werbotv1connect.UnimplementedResourcePresetServiceHandler
	svc *Service
}

// NewPresetHandler returns the Connect handler for ResourcePresetService.
func NewPresetHandler(s *Service) *PresetHandler { return &PresetHandler{svc: s} }

func portsToProto(rs []connector.PortRange) []*connectorv1.PortRange {
	out := make([]*connectorv1.PortRange, len(rs))
	for i, r := range rs {
		out[i] = &connectorv1.PortRange{Port: uint32(r.Port), EndPort: uint32(r.EndPort)}
	}
	return out
}

func presetToProto(p Preset) *werbotv1.ResourcePreset {
	return &werbotv1.ResourcePreset{Id: p.ID.String(), Name: p.Name, BaseType: p.BaseType, DisplayName: p.DisplayName, Icon: p.Icon,
		DefaultPorts: portsToProto(p.DefaultPorts), Category: p.Category, LaunchHint: p.LaunchHint, CreatedAt: timestamppb.New(p.CreatedAt)}
}

// presetTypeToProto is the base type's entry with the preset's name, category and display fields.
func presetTypeToProto(p Preset, base connector.Type) *werbotv1.ResourceType {
	out := typeToProto(base)
	out.Name, out.Category = p.Name, p.Category
	out.PresetId, out.BaseType, out.DisplayName, out.Icon = p.ID.String(), p.BaseType, p.DisplayName, p.Icon
	out.DefaultPorts, out.LaunchHint = portsToProto(p.DefaultPorts), p.LaunchHint
	return out
}

func presetInput(name, baseType, displayName, icon, category, hint string, ports []*connectorv1.PortRange) (PresetInput, error) {
	rs, err := connector.PortsFromProto(ports)
	if err != nil {
		return PresetInput{}, invalid("default_ports: %v", err)
	}
	return PresetInput{Name: name, BaseType: baseType, DisplayName: displayName, Icon: icon, Category: category, LaunchHint: hint, DefaultPorts: rs}, nil
}

// ListResourcePresets lists the presets of the organization by name.
func (h *PresetHandler) ListResourcePresets(ctx context.Context, _ *connect.Request[werbotv1.ListResourcePresetsRequest]) (*connect.Response[werbotv1.ListResourcePresetsResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	ps, err := h.svc.ListPresets(ctx, org)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListResourcePresetsResponse{}
	for _, p := range ps {
		res.Presets = append(res.Presets, presetToProto(p))
	}
	return connect.NewResponse(res), nil
}

// CreateResourcePreset adds a preset.
func (h *PresetHandler) CreateResourcePreset(ctx context.Context, req *connect.Request[werbotv1.CreateResourcePresetRequest]) (*connect.Response[werbotv1.CreateResourcePresetResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	in, err := presetInput(m.GetName(), m.GetBaseType(), m.GetDisplayName(), m.GetIcon(), m.GetCategory(), m.GetLaunchHint(), m.GetDefaultPorts())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	p, err := h.svc.CreatePreset(ctx, org, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreateResourcePresetResponse{Preset: presetToProto(p)}), nil
}

// UpdateResourcePreset replaces the display fields; name and base_type must stay as stored.
func (h *PresetHandler) UpdateResourcePreset(ctx context.Context, req *connect.Request[werbotv1.UpdateResourcePresetRequest]) (*connect.Response[werbotv1.UpdateResourcePresetResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	id, err := parseID(m.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	in, err := presetInput(m.GetName(), m.GetBaseType(), m.GetDisplayName(), m.GetIcon(), m.GetCategory(), m.GetLaunchHint(), m.GetDefaultPorts())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	p, err := h.svc.UpdatePreset(ctx, org, id, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.UpdateResourcePresetResponse{Preset: presetToProto(p)}), nil
}

// DeleteResourcePreset removes a preset no resource uses.
func (h *PresetHandler) DeleteResourcePreset(ctx context.Context, req *connect.Request[werbotv1.DeleteResourcePresetRequest]) (*connect.Response[werbotv1.DeleteResourcePresetResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.DeletePreset(ctx, org, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteResourcePresetResponse{}), nil
}

// presetType returns the preset id of a create request, nil without one. The
// preset must belong to the organization and the type must equal its base_type.
func (h *Handler) presetType(ctx context.Context, org uuid.UUID, m *werbotv1.CreateResourceRequest) (*uuid.UUID, error) {
	if m.GetPresetId() == "" {
		return nil, nil
	}
	id, err := parseID(m.GetPresetId())
	if err != nil {
		return nil, err
	}
	p, err := h.svc.GetPreset(ctx, org, id)
	if errors.Is(err, apperr.ErrNotFound) {
		return nil, invalid("preset not found")
	}
	if err != nil {
		return nil, err
	}
	if m.GetType() != p.BaseType {
		return nil, invalid("type must equal the base type %q of the preset", p.BaseType)
	}
	return &id, nil
}
