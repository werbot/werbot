//go:build cluster || load || redteam

package cluster

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
)

// sessionCookie is the session cookie name of WERBOT_ENV=development (production: __Host-werbot_session, contract §3.1).
const sessionCookie = "werbot_session"

// API is a set of Connect clients on one base URL with one session cookie.
type API struct {
	BaseURL   string
	HTTP      *http.Client // carries the cookie: use it for plain HTTP such as /api/recordings/{id}.cast
	Auth      werbotv1connect.AuthServiceClient
	SSHKeys   werbotv1connect.SSHKeyServiceClient
	Gateways  werbotv1connect.GatewayServiceClient
	Enroll    werbotv1connect.GatewayEnrollServiceClient
	Resources werbotv1connect.ResourceServiceClient
	Access    werbotv1connect.AccessServiceClient
	Policy    werbotv1connect.PolicyServiceClient
	Sessions  werbotv1connect.SessionServiceClient
	System    werbotv1connect.SystemServiceClient

	AccessRequests   werbotv1connect.AccessRequestServiceClient
	ApprovalPolicies werbotv1connect.ApprovalPolicyServiceClient
	Agents           werbotv1connect.AgentServiceClient
	Locks            werbotv1connect.LockServiceClient
	Members          werbotv1connect.MemberServiceClient
	Settings         werbotv1connect.SettingsServiceClient
	OAuthConsent     werbotv1connect.OAuthConsentServiceClient
}

// NewAPI builds clients for baseURL; a non-empty cookie is sent as the session.
// Hosts under .werbot.test dial 127.0.0.1, so the Host header stays the stand's name.
func (*Stand) NewAPI(baseURL, cookie string) API {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	var d net.Dialer
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if host, port, err := net.SplitHostPort(addr); err == nil && strings.HasSuffix(host, ".werbot.test") {
			addr = net.JoinHostPort("127.0.0.1", port)
		}
		return d.DialContext(ctx, network, addr)
	}
	jar, _ := cookiejar.New(nil) // error is always nil
	if u, err := url.Parse(baseURL); err == nil && cookie != "" {
		jar.SetCookies(u, []*http.Cookie{{Name: sessionCookie, Value: cookie, Path: "/"}}) //nolint:gosec // client jar entry; the plain-http development stand never sets Secure
	}
	hc := &http.Client{Transport: tr, Jar: jar, Timeout: 30 * time.Second}
	return API{
		BaseURL:   baseURL,
		HTTP:      hc,
		Auth:      werbotv1connect.NewAuthServiceClient(hc, baseURL),
		SSHKeys:   werbotv1connect.NewSSHKeyServiceClient(hc, baseURL),
		Gateways:  werbotv1connect.NewGatewayServiceClient(hc, baseURL),
		Enroll:    werbotv1connect.NewGatewayEnrollServiceClient(hc, baseURL),
		Resources: werbotv1connect.NewResourceServiceClient(hc, baseURL),
		Access:    werbotv1connect.NewAccessServiceClient(hc, baseURL),
		Policy:    werbotv1connect.NewPolicyServiceClient(hc, baseURL),
		Sessions:  werbotv1connect.NewSessionServiceClient(hc, baseURL),
		System:    werbotv1connect.NewSystemServiceClient(hc, baseURL),

		AccessRequests:   werbotv1connect.NewAccessRequestServiceClient(hc, baseURL),
		ApprovalPolicies: werbotv1connect.NewApprovalPolicyServiceClient(hc, baseURL),
		Agents:           werbotv1connect.NewAgentServiceClient(hc, baseURL),
		Locks:            werbotv1connect.NewLockServiceClient(hc, baseURL),
		Members:          werbotv1connect.NewMemberServiceClient(hc, baseURL),
		Settings:         werbotv1connect.NewSettingsServiceClient(hc, baseURL),
		OAuthConsent:     werbotv1connect.NewOAuthConsentServiceClient(hc, baseURL),
	}
}

// SessionCookie returns the session cookie value set by a response, or "".
func SessionCookie(h http.Header) (cookie string) {
	for _, line := range h.Values("Set-Cookie") {
		if c, err := http.ParseSetCookie(line); err == nil && c.Name == sessionCookie {
			cookie = c.Value
		}
	}
	return cookie
}

// signIn runs the first-run Setup for alice through nginx and builds Edge, A, B and UI with her cookie.
func (s *Stand) signIn(ctx context.Context) error {
	edge, err := s.port(ctx, "nginx", 80)
	if err != nil {
		return err
	}
	res, err := s.NewAPI("http://"+edge, "").Auth.Setup(ctx, connect.NewRequest(&werbotv1.SetupRequest{
		Email: "alice@example.com", Login: "alice", Name: "Alice", Password: "cluster-password-42", OrganizationName: "Cluster",
	}))
	if err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	cookie := SessionCookie(res.Header())
	if cookie == "" {
		return errors.New("setup: no session cookie")
	}
	a, err := s.port(ctx, "nginx", 8081)
	if err != nil {
		return err
	}
	b, err := s.port(ctx, "nginx", 8082)
	if err != nil {
		return err
	}
	_, port, _ := net.SplitHostPort(edge) // edge comes from docker compose port: host:port
	s.Edge, s.A, s.B = s.NewAPI("http://"+edge, cookie), s.NewAPI("http://"+a, cookie), s.NewAPI("http://"+b, cookie)
	s.UI = s.NewAPI("http://"+net.JoinHostPort(UIHost, port), cookie)
	me, err := s.Edge.Auth.GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}
	if s.AliceID, err = uuid.Parse(me.Msg.GetUser().GetId()); err != nil {
		return fmt.Errorf("alice id: %w", err)
	}
	if s.OrgID, err = uuid.Parse(me.Msg.GetOrganization().GetId()); err != nil {
		return fmt.Errorf("org id: %w", err)
	}
	return nil
}

// enrollPool creates pool-a and a two-use join token, starts gw1 and gw2 with it
// and waits until both are ready and online.
func (s *Stand) enrollPool(ctx context.Context) error {
	gw, err := s.Edge.Gateways.CreateGateway(ctx, connect.NewRequest(&werbotv1.CreateGatewayRequest{Name: "pool-a"}))
	if err != nil {
		return fmt.Errorf("create gateway: %w", err)
	}
	if s.PoolID, err = uuid.Parse(gw.Msg.GetGateway().GetId()); err != nil {
		return fmt.Errorf("pool id: %w", err)
	}
	tok, err := s.Edge.Gateways.CreateJoinToken(ctx, connect.NewRequest(&werbotv1.CreateJoinTokenRequest{
		GatewayId: s.PoolID.String(), MaxUses: 2, ExpiresIn: durationpb.New(time.Hour),
	}))
	if err != nil {
		return fmt.Errorf("create join token: %w", err)
	}
	s.env["JOIN_TOKEN"] = tok.Msg.GetToken()
	if _, err := s.Compose(ctx, "up", "-d", "gw1", "gw2"); err != nil {
		return err
	}
	return poll(ctx, time.Minute, func(ctx context.Context) error {
		if err := s.ready(ctx, "gw1", "gw2"); err != nil {
			return err
		}
		return s.poolOnline(ctx)
	})
}

// poolOnline returns nil when pool-a has exactly two online instances.
func (s *Stand) poolOnline(ctx context.Context) error {
	res, err := s.Edge.Gateways.ListGateways(ctx, connect.NewRequest(&werbotv1.ListGatewaysRequest{}))
	if err != nil {
		return fmt.Errorf("list gateways: %w", err)
	}
	for _, g := range res.Msg.GetGateways() {
		if g.GetName() != "pool-a" {
			continue
		}
		var online []string
		for _, in := range g.GetInstances() {
			if in.GetOnline() {
				online = append(online, in.GetHostname())
			}
		}
		if len(online) != 2 {
			return fmt.Errorf("pool-a online instances %v, want 2", online)
		}
		return nil
	}
	return errors.New("no pool-a")
}

// addResources creates the SSH resources on pool-a, binds alice to target and
// geo, and allows geo only from NL.
func (s *Stand) addResources(ctx context.Context) error {
	cfg, err := anypb.New(&sshv1.Config{Host: "target", Port: 2222, Login: "werbot"})
	if err != nil {
		return err
	}
	for _, name := range []string{"target", "target2", "geo"} {
		res, err := s.Edge.Resources.CreateResource(ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{
			Name: name, Type: "ssh", GatewayId: s.PoolID.String(), Config: cfg,
		}))
		if err != nil {
			return fmt.Errorf("create resource %s: %w", name, err)
		}
		id := res.Msg.GetResource().GetId()
		if s.Resources[name], err = uuid.Parse(id); err != nil {
			return fmt.Errorf("resource %s id: %w", name, err)
		}
		if _, err := s.Edge.Resources.SetCredential(ctx, connect.NewRequest(&werbotv1.SetCredentialRequest{
			ResourceId: id, Kind: "password", Value: []byte("werbot-target"),
		})); err != nil {
			return fmt.Errorf("set credential %s: %w", name, err)
		}
		if name == "target2" {
			continue
		}
		if _, err := s.Edge.Access.CreateBinding(ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{
			SubjectType: "user", SubjectId: s.AliceID.String(), ResourceId: id,
		})); err != nil {
			return fmt.Errorf("bind %s: %w", name, err)
		}
	}
	_, err = s.Edge.Policy.SetResourceRules(ctx, connect.NewRequest(&werbotv1.SetResourceRulesRequest{
		ResourceId: s.Resources["geo"].String(), Countries: []*werbotv1.CountryRule{{Country: "NL", Effect: "allow"}},
	}))
	if err != nil {
		return fmt.Errorf("geo rules: %w", err)
	}
	return nil
}
