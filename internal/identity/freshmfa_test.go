package identity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// adminAction is the procedure the tests guard with RequireFreshMFA.
const adminAction = werbotv1connect.MemberServiceInviteMemberProcedure

// procRequest is req with its Spec naming proc.
type procRequest[T any] struct {
	*connect.Request[T]
	proc string
}

func (r procRequest[T]) Spec() connect.Spec { return connect.Spec{Procedure: r.proc} }

// throughFresh runs proc with msg through RequireFreshMFA({adminAction}) in
// ctx and returns the interceptor's error (nil: the handler ran).
func throughFresh[T any](ctx context.Context, proc string, msg *T) error {
	ran := errors.New("handler ran")
	next := connect.UnaryFunc(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) { return nil, ran })
	req := procRequest[T]{Request: connect.NewRequest(msg), proc: proc}
	_, err := RequireFreshMFA(map[string]bool{adminAction: true}).WrapUnary(next)(ctx, req)
	if errors.Is(err, ran) {
		return nil
	}
	return err
}

// assertRefused checks a PermissionDenied answer with text.
func assertRefused(t *testing.T, err error, text string, msgAndArgs ...any) {
	t.Helper()
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), msgAndArgs...)
	var ce *connect.Error
	if assert.ErrorAs(t, err, &ce, msgAndArgs...) {
		assert.Equal(t, text, ce.Message(), msgAndArgs...)
	}
}

// stateCtx is a request context of a role's principal with MFA state: mode
// is require_mfa, ago how long ago the credential passed a factor (< 0: never).
func stateCtx(role, mode string, factor bool, ago time.Duration) context.Context {
	st := mfaState{RequireMFA: mode, HasFactor: factor}
	if ago >= 0 {
		at := time.Now().Add(-ago)
		st.FreshAt = &at
	}
	p := principal.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: role, AuthMethod: principal.AuthMethodCookie, SessionID: uuid.New()}
	return withMFAState(principal.With(context.Background(), p), st)
}

func TestRequireFreshMFA(t *testing.T) {
	const never = -1
	other := werbotv1connect.MemberServiceListMembersProcedure
	agent := uuid.New()
	agentCtx := stateCtx(principal.RoleOwner, "off", true, time.Minute)
	p, _ := principal.From(agentCtx)
	p.AgentID, p.RunID = &agent, &agent
	agentCtx = principal.With(agentCtx, p)

	for _, c := range []struct {
		name, proc, want string
		ctx              context.Context
	}{
		{"no factor, off: owner", adminAction, "", stateCtx(principal.RoleOwner, "off", false, never)},
		{"no factor, off: member", adminAction, "", stateCtx(principal.RoleMember, "off", false, never)},
		{"factor, never verified", adminAction, "mfa required", stateCtx(principal.RoleOwner, "off", true, never)},
		{"factor, 6 minutes ago", adminAction, "mfa required", stateCtx(principal.RoleOwner, "off", true, 6*time.Minute)},
		{"factor, 4 minutes ago", adminAction, "", stateCtx(principal.RoleOwner, "off", true, 4*time.Minute)},
		{"factor, stale, procedure not listed", other, "", stateCtx(principal.RoleOwner, "off", true, 6*time.Minute)},
		// The enrollment gate answers first: the admin has no factor to step up with.
		{"no factor, admins: admin", adminAction, "mfa enrollment required", stateCtx(principal.RoleAdmin, "admins", false, never)},
		{"no factor, admins: member", adminAction, "", stateCtx(principal.RoleMember, "admins", false, never)},
		{"factor, fresh: agent token", adminAction, "mfa required", agentCtx},
		{"no principal", adminAction, "", context.Background()},
	} {
		err := throughFresh(c.ctx, c.proc, &werbotv1.InviteMemberRequest{})
		if c.want == "" {
			assert.NoError(t, err, c.name)
			continue
		}
		assertRefused(t, err, c.want, c.name)
	}
}

func TestRequireFreshMFABreakGlass(t *testing.T) {
	proc := werbotv1connect.AccessRequestServiceCreateRequestProcedure
	stale := stateCtx(principal.RoleMember, "off", true, 6*time.Minute)
	require.NoError(t, throughFresh(stale, proc, &werbotv1.CreateRequestRequest{}), "an ordinary request")
	assertRefused(t, throughFresh(stale, proc, &werbotv1.CreateRequestRequest{BreakGlass: true}), "mfa required")
	require.NoError(t, throughFresh(stateCtx(principal.RoleMember, "off", true, time.Minute), proc, &werbotv1.CreateRequestRequest{BreakGlass: true}))
	require.NoError(t, throughFresh(stateCtx(principal.RoleMember, "off", false, -1), proc, &werbotv1.CreateRequestRequest{BreakGlass: true}),
		"no factor and require_mfa off")
}

func TestRequireFreshMFABearer(t *testing.T) {
	s := newCLIStand(t)
	org := seedOrg(t, s.pool, "Acme")
	alice := seedUser(t, s.pool, org, "alice", principal.RoleAdmin)
	secret := dbtest.Fixture{}.TOTP(t, s.pool, s.svc.cfg.Box, alice)
	bearerCtx := func(token string) (context.Context, principal.Principal) {
		t.Helper()
		p, st, err := s.svc.authenticateBearer(context.Background(), s.prov, "Bearer "+token)
		require.NoError(t, err)
		return withMFAState(principal.With(context.Background(), p), st), p
	}
	first := s.cliLogin(t, org, alice).AccessToken
	second := s.cliLogin(t, org, alice).AccessToken
	ctx, p := bearerCtx(first)
	assertRefused(t, throughFresh(ctx, adminAction, &werbotv1.InviteMemberRequest{}), "mfa required", "no check of this token yet")

	require.NoError(t, s.svc.CompleteStepUp(ctx, p, MFAProof{TOTP: codeAt(t, secret, currentStep())}))
	ctx, _ = bearerCtx(first)
	require.NoError(t, throughFresh(ctx, adminAction, &werbotv1.InviteMemberRequest{}))
	ctx, _ = bearerCtx(second)
	assertRefused(t, throughFresh(ctx, adminAction, &werbotv1.InviteMemberRequest{}), "mfa required", "another CLI session of the user")
}

// freshStand serves AuthService, MFAService and MemberService behind
// Middleware with RequireFreshMFA({adminAction}).
func freshStand(t *testing.T, svc *Service) (werbotv1connect.AuthServiceClient, werbotv1connect.MFAServiceClient, werbotv1connect.MemberServiceClient) {
	t.Helper()
	h := NewHandler(svc)
	opts := connect.WithInterceptors(RequireFreshMFA(map[string]bool{adminAction: true}))
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewAuthServiceHandler(h, opts))
	mux.Handle(werbotv1connect.NewMFAServiceHandler(h, opts))
	mux.Handle(werbotv1connect.NewMemberServiceHandler(h, opts))
	srv := httptest.NewServer(Middleware(svc, nil)(mux))
	t.Cleanup(srv.Close)
	hc := srv.Client()
	return werbotv1connect.NewAuthServiceClient(hc, srv.URL), werbotv1connect.NewMFAServiceClient(hc, srv.URL),
		werbotv1connect.NewMemberServiceClient(hc, srv.URL)
}

// cookieReq is msg sent with the secure session cookie token.
func cookieReq[T any](msg *T, token string) *connect.Request[T] {
	return withCookie(msg, "__Host-werbot_session", token)
}

// enroll enrolls and confirms a TOTP factor over mfa with token and returns its secret.
func enroll(t *testing.T, mfa werbotv1connect.MFAServiceClient, token string) string {
	t.Helper()
	ctx := context.Background()
	e, err := mfa.EnrollTOTP(ctx, cookieReq(&werbotv1.EnrollTOTPRequest{}, token))
	require.NoError(t, err, "the first enrollment needs no step-up")
	_, err = mfa.ConfirmTOTP(ctx, cookieReq(&werbotv1.ConfirmTOTPRequest{Code: codeAt(t, e.Msg.GetSecret(), currentStep())}, token))
	require.NoError(t, err)
	return e.Msg.GetSecret()
}

func TestEnrollmentGate(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleAdmin)
	auth, mfa, members := freshStand(t, svc)
	_, token := loginCtx(t, svc, "alice")
	setRequireMFA(t, svc, "all")
	ctx := context.Background()

	_, err := members.ListMembers(ctx, cookieReq(&werbotv1.ListMembersRequest{}, token))
	assertRefused(t, err, "mfa enrollment required")
	_, err = auth.GetSession(ctx, cookieReq(&werbotv1.GetSessionRequest{}, token))
	require.NoError(t, err)
	enroll(t, mfa, token)
	_, err = members.ListMembers(ctx, cookieReq(&werbotv1.ListMembersRequest{}, token))
	require.NoError(t, err, "enrolled")
}

func TestMFAServiceRequiresFreshWhenEnrolled(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleMember)
	auth, mfa, _ := freshStand(t, svc)
	_, token := loginCtx(t, svc, "alice")
	ctx := context.Background()
	secret := enroll(t, mfa, token)
	_, err := pool.Exec(ctx, "UPDATE user_session SET mfa_at = now() - interval '6 minutes'")
	require.NoError(t, err)

	refused := map[string]error{}
	_, refused["EnrollTOTP"] = mfa.EnrollTOTP(ctx, cookieReq(&werbotv1.EnrollTOTPRequest{}, token))
	_, refused["DeleteFactor"] = mfa.DeleteFactor(ctx, cookieReq(&werbotv1.DeleteFactorRequest{Id: "totp"}, token))
	_, refused["GenerateRecoveryCodes"] = mfa.GenerateRecoveryCodes(ctx, cookieReq(&werbotv1.GenerateRecoveryCodesRequest{}, token))
	_, refused["BeginWebAuthnRegistration"] = mfa.BeginWebAuthnRegistration(ctx, cookieReq(&werbotv1.BeginWebAuthnRegistrationRequest{}, token))
	for name, err := range refused {
		assertRefused(t, err, "mfa required", name)
	}
	_, err = mfa.ListFactors(ctx, cookieReq(&werbotv1.ListFactorsRequest{}, token))
	require.NoError(t, err, "reading factors needs no step-up")

	_, err = auth.CompleteStepUp(ctx, cookieReq(&werbotv1.CompleteStepUpRequest{
		Proof: &werbotv1.CompleteStepUpRequest_TotpCode{TotpCode: codeAt(t, secret, currentStep()+1)}}, token))
	require.NoError(t, err)
	_, err = mfa.GenerateRecoveryCodes(ctx, cookieReq(&werbotv1.GenerateRecoveryCodesRequest{}, token))
	require.NoError(t, err, "after a step-up")
}
