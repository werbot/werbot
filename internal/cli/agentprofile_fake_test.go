package cli

import (
	"context"
	"net/http"
	"sync"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
)

// profileToken is the refresh token the fake server hands out for a new agent profile.
const profileToken = "wrt_profile-secret" //nolint:gosec // test value

// profileFake stands in for AgentProfileService and /oauth/revoke.
type profileFake struct {
	werbotv1connect.UnimplementedAgentProfileServiceHandler

	pmu           sync.Mutex
	profiles      []*werbotv1.AgentProfile
	created       []*werbotv1.CreateAgentProfileRequest
	revoked       []string // profile ids
	revokedTokens []string // refresh tokens given to /oauth/revoke
	newAgents     []string // names given to CreateAgent
}

func (f *profileFake) CreateAgentProfile(_ context.Context, req *connect.Request[werbotv1.CreateAgentProfileRequest]) (*connect.Response[werbotv1.CreateAgentProfileResponse], error) {
	f.pmu.Lock()
	defer f.pmu.Unlock()
	f.created = append(f.created, req.Msg)
	p := &werbotv1.AgentProfile{Id: uuid.NewString(), AgentId: req.Msg.AgentId, Label: req.Msg.Label}
	f.profiles = append(f.profiles, p)
	return connect.NewResponse(&werbotv1.CreateAgentProfileResponse{Profile: p, RefreshToken: profileToken}), nil
}

func (f *profileFake) ListAgentProfiles(context.Context, *connect.Request[werbotv1.ListAgentProfilesRequest]) (*connect.Response[werbotv1.ListAgentProfilesResponse], error) {
	f.pmu.Lock()
	defer f.pmu.Unlock()
	return connect.NewResponse(&werbotv1.ListAgentProfilesResponse{Profiles: f.profiles}), nil
}

func (f *profileFake) RevokeAgentProfile(_ context.Context, req *connect.Request[werbotv1.RevokeAgentProfileRequest]) (*connect.Response[werbotv1.RevokeAgentProfileResponse], error) {
	f.pmu.Lock()
	defer f.pmu.Unlock()
	f.revoked = append(f.revoked, req.Msg.Id)
	return connect.NewResponse(&werbotv1.RevokeAgentProfileResponse{}), nil
}

func (f *profileFake) revokeToken(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.pmu.Lock()
	f.revokedTokens = append(f.revokedTokens, r.PostForm.Get("token"))
	f.pmu.Unlock()
	w.WriteHeader(http.StatusOK)
}

// CreateAgent answers with the agent the fake already knows by id.
func (f *agentFake) CreateAgent(_ context.Context, req *connect.Request[werbotv1.CreateAgentRequest]) (*connect.Response[werbotv1.CreateAgentResponse], error) {
	f.pmu.Lock()
	f.newAgents = append(f.newAgents, req.Msg.Name)
	f.pmu.Unlock()
	return connect.NewResponse(&werbotv1.CreateAgentResponse{Agent: &werbotv1.Agent{Id: f.agentID.String(), Name: req.Msg.Name}}), nil
}

// pick reads one list of the fake under its lock.
func (f *profileFake) pick(get func() []string) []string {
	f.pmu.Lock()
	defer f.pmu.Unlock()
	return append([]string(nil), get()...)
}

func (f *profileFake) createdProfiles() []*werbotv1.CreateAgentProfileRequest {
	f.pmu.Lock()
	defer f.pmu.Unlock()
	return append([]*werbotv1.CreateAgentProfileRequest(nil), f.created...)
}
