package identity

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

func settingsToProto(o OrgSettings) *werbotv1.OrgSettings {
	return &werbotv1.OrgSettings{CliSessionTtl: durationpb.New(o.CLISessionTTL), CliSessionMax: durationpb.New(o.CLISessionMax),
		GrantDefaultTtl: durationpb.New(o.GrantDefaultTTL), GrantMaxTtl: durationpb.New(o.GrantMaxTTL),
		RequestPendingTtl: durationpb.New(o.RequestPendingTTL), CommandApprovalTtl: durationpb.New(o.CommandApprovalTTL), LockMode: o.LockMode, MaskOutput: o.MaskOutput,
		RequireMfa: o.RequireMFA}
}

// GetSettings returns the organization's settings.
func (h *Handler) GetSettings(ctx context.Context, _ *connect.Request[werbotv1.GetSettingsRequest]) (*connect.Response[werbotv1.GetSettingsResponse], error) {
	p, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	o, err := h.svc.Settings(ctx, p.OrgID)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.GetSettingsResponse{Settings: settingsToProto(o)}), nil
}

// UpdateSettings writes the settings fields named by update_mask.
func (h *Handler) UpdateSettings(ctx context.Context, req *connect.Request[werbotv1.UpdateSettingsRequest]) (*connect.Response[werbotv1.UpdateSettingsResponse], error) {
	p, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	in := req.Msg.GetSettings()
	o, err := h.svc.UpdateSettings(ctx, p, OrgSettings{
		CLISessionTTL: in.GetCliSessionTtl().AsDuration(), CLISessionMax: in.GetCliSessionMax().AsDuration(),
		GrantDefaultTTL: in.GetGrantDefaultTtl().AsDuration(), GrantMaxTTL: in.GetGrantMaxTtl().AsDuration(),
		RequestPendingTTL: in.GetRequestPendingTtl().AsDuration(), CommandApprovalTTL: in.GetCommandApprovalTtl().AsDuration(),
		LockMode: in.GetLockMode(), MaskOutput: in.GetMaskOutput(), RequireMFA: in.GetRequireMfa(),
	}, req.Msg.GetUpdateMask().GetPaths())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.UpdateSettingsResponse{Settings: settingsToProto(o)}), nil
}
