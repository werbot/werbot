package oauth

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// AgentProfileHandler serves AgentProfileService.
type AgentProfileHandler struct{ p *Provider }

var _ werbotv1connect.AgentProfileServiceHandler = (*AgentProfileHandler)(nil)

// NewAgentProfileHandler returns the AgentProfileService handler of p.
func NewAgentProfileHandler(p *Provider) *AgentProfileHandler { return &AgentProfileHandler{p: p} }

// CreateAgentProfile opens a profile of the caller's agent; the response
// carries its refresh token once.
func (h *AgentProfileHandler) CreateAgentProfile(ctx context.Context, req *connect.Request[werbotv1.CreateAgentProfileRequest]) (*connect.Response[werbotv1.CreateAgentProfileResponse], error) {
	pr, agentID, err := subjectAndID(ctx, req.Msg.GetAgentId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	prof, token, err := h.p.CreateAgentProfile(ctx, pr, agentID, req.Msg.GetTtl().AsDuration(), req.Msg.GetLabel())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreateAgentProfileResponse{Profile: profileMsg(prof), RefreshToken: token}), nil
}

// ListAgentProfiles lists the live profiles the caller sees, of one agent if set.
func (h *AgentProfileHandler) ListAgentProfiles(ctx context.Context, req *connect.Request[werbotv1.ListAgentProfilesRequest]) (*connect.Response[werbotv1.ListAgentProfilesResponse], error) {
	pr, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	var agentID *uuid.UUID
	if s := req.Msg.GetAgentId(); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, apperr.ToConnect(apperr.ErrInvalidArgument)
		}
		agentID = &id
	}
	list, err := h.p.ListAgentProfiles(ctx, pr, agentID)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	out := make([]*werbotv1.AgentProfile, len(list))
	for i, prof := range list {
		out[i] = profileMsg(prof)
	}
	return connect.NewResponse(&werbotv1.ListAgentProfilesResponse{Profiles: out}), nil
}

// RevokeAgentProfile ends a profile.
func (h *AgentProfileHandler) RevokeAgentProfile(ctx context.Context, req *connect.Request[werbotv1.RevokeAgentProfileRequest]) (*connect.Response[werbotv1.RevokeAgentProfileResponse], error) {
	pr, id, err := subjectAndID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.p.RevokeAgentProfile(ctx, pr, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.RevokeAgentProfileResponse{}), nil
}

func profileMsg(p AgentProfile) *werbotv1.AgentProfile {
	return &werbotv1.AgentProfile{
		Id: p.ID.String(), AgentId: p.AgentID.String(), AgentName: p.AgentName, SponsorUserId: p.SponsorID.String(), Label: p.Label,
		CreatedAt: timestamppb.New(p.CreatedAt), LastUsedAt: timestamppb.New(p.LastUsedAt),
		ExpiresAt: timestamppb.New(p.ExpiresAt), AbsoluteExpiresAt: timestamppb.New(p.AbsoluteExpiresAt),
	}
}
