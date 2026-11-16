package identity

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

func teamToProto(t Team) *werbotv1.Team {
	ids := make([]string, len(t.MemberUserIDs))
	for i, id := range t.MemberUserIDs {
		ids[i] = id.String()
	}
	return &werbotv1.Team{Id: t.ID.String(), Name: t.Name, MemberUserIds: ids, CreatedAt: timestamppb.New(t.CreatedAt)}
}

// requireTeamAdmin is the session check every TeamService method starts with.
func requireTeamAdmin(ctx context.Context) error {
	_, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin)
	return apperr.ToConnect(err)
}

// ListTeams returns the organization's teams.
func (h *Handler) ListTeams(ctx context.Context, _ *connect.Request[werbotv1.ListTeamsRequest]) (*connect.Response[werbotv1.ListTeamsResponse], error) {
	if err := requireTeamAdmin(ctx); err != nil {
		return nil, err
	}
	teams, err := h.svc.ListTeams(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListTeamsResponse{}
	for _, t := range teams {
		res.Teams = append(res.Teams, teamToProto(t))
	}
	return connect.NewResponse(res), nil
}

// CreateTeam creates a team.
func (h *Handler) CreateTeam(ctx context.Context, req *connect.Request[werbotv1.CreateTeamRequest]) (*connect.Response[werbotv1.CreateTeamResponse], error) {
	if err := requireTeamAdmin(ctx); err != nil {
		return nil, err
	}
	t, err := h.svc.CreateTeam(ctx, req.Msg.GetName())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreateTeamResponse{Team: teamToProto(t)}), nil
}

// RenameTeam renames a team.
func (h *Handler) RenameTeam(ctx context.Context, req *connect.Request[werbotv1.RenameTeamRequest]) (*connect.Response[werbotv1.RenameTeamResponse], error) {
	if err := requireTeamAdmin(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	t, err := h.svc.RenameTeam(ctx, id, req.Msg.GetName())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.RenameTeamResponse{Team: teamToProto(t)}), nil
}

// DeleteTeam deletes a team.
func (h *Handler) DeleteTeam(ctx context.Context, req *connect.Request[werbotv1.DeleteTeamRequest]) (*connect.Response[werbotv1.DeleteTeamResponse], error) {
	if err := requireTeamAdmin(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.DeleteTeam(ctx, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteTeamResponse{}), nil
}

func parseTeamUser(teamID, userID string) (uuid.UUID, uuid.UUID, error) {
	t, err := parseID(teamID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	u, err := parseID(userID)
	return t, u, err
}

// AddTeamMember adds a member to a team.
func (h *Handler) AddTeamMember(ctx context.Context, req *connect.Request[werbotv1.AddTeamMemberRequest]) (*connect.Response[werbotv1.AddTeamMemberResponse], error) {
	if err := requireTeamAdmin(ctx); err != nil {
		return nil, err
	}
	teamID, userID, err := parseTeamUser(req.Msg.GetTeamId(), req.Msg.GetUserId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.AddTeamMember(ctx, teamID, userID); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.AddTeamMemberResponse{}), nil
}

// RemoveTeamMember removes a member from a team.
func (h *Handler) RemoveTeamMember(ctx context.Context, req *connect.Request[werbotv1.RemoveTeamMemberRequest]) (*connect.Response[werbotv1.RemoveTeamMemberResponse], error) {
	if err := requireTeamAdmin(ctx); err != nil {
		return nil, err
	}
	teamID, userID, err := parseTeamUser(req.Msg.GetTeamId(), req.Msg.GetUserId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.RemoveTeamMember(ctx, teamID, userID); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.RemoveTeamMemberResponse{}), nil
}
