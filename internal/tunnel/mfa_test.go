package tunnel

import (
	"math"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// keyBinding is a key binding as the gateway computes it.
var keyBinding = "key:SHA256:" + strings.Repeat("A", 43)

func challengeReq(user, resource uuid.UUID) *gatewayv1.CreateMFAChallengeRequest {
	return &gatewayv1.CreateMFAChallengeRequest{UserId: user.String(), ResourceId: resource.String(), SshSessionId: []byte("s1"),
		ClientIp: "198.51.100.4", Binding: keyBinding, Strength: "any"}
}

// createChallenge creates a challenge of user on res as g and returns its id.
func createChallenge(t *testing.T, g *gw, user, res uuid.UUID) string {
	t.Helper()
	resp, err := g.client.CreateMFAChallenge(ctx, connect.NewRequest(challengeReq(user, res)))
	if err != nil {
		t.Fatal(err)
	}
	m := resp.Msg
	if m.GetCode() != "BCDF-GHJK" || m.GetUrl() != "https://werbot.test/mfa/"+m.GetChallengeId() || m.GetExpiresAt().AsTime().Unix() != 1767225600 {
		t.Fatalf("response %+v", m)
	}
	return m.GetChallengeId()
}

// verifyTOTP calls VerifyMFATOTP as g and returns ok, failing on an error of another code than want.
func verifyTOTP(t *testing.T, g *gw, id, code string, want connect.Code) bool {
	t.Helper()
	r, err := g.client.VerifyMFATOTP(ctx, connect.NewRequest(&gatewayv1.VerifyMFATOTPRequest{ChallengeId: id, Code: code}))
	if want != 0 {
		wantCode(t, err, want, "")
		return false
	}
	if err != nil {
		t.Fatalf("code %s: %v", code, err)
	}
	return r.Msg.GetOk()
}

// Challenge RPCs take the organization and instance from the certificate:
// another instance's challenge is not found, a wrong code is ok=false and
// the attempt limit is ResourceExhausted.
func TestMFAChallengeRPCs(t *testing.T) {
	b, _, fm := newMFABench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	peer := b.instance(t, b.org, g.ID)
	b.f.with(func(f *fake) { f.owned = true })
	user, res := uuid.New(), uuid.New()
	id := createChallenge(t, g, user, res)
	want := MFAChallengeRequest{OrgID: b.org, InstanceID: g.InstanceID, UserID: user, ResourceID: res, SSHSessionID: []byte("s1"),
		ClientIP: netip.MustParseAddr("198.51.100.4"), Binding: keyBinding, Strength: "any"}
	fm.with(func(m *fakeMFA) {
		if !reflect.DeepEqual(m.creates, []MFAChallengeRequest{want}) {
			t.Fatalf("Create(%+v), want org and instance from the certificate and the request fields", m.creates)
		}
	})

	verifyTOTP(t, peer, id, "123456", connect.CodeNotFound)
	_, err := peer.client.GetMFAChallenge(ctx, connect.NewRequest(&gatewayv1.GetMFAChallengeRequest{ChallengeId: id}))
	wantCode(t, err, connect.CodeNotFound, "")
	verifyTOTP(t, g, id, exhaustedTOTP, connect.CodeResourceExhausted)
	verifyTOTP(t, g, id, "12345", connect.CodeInvalidArgument)
	if got := []bool{verifyTOTP(t, g, id, wrongTOTP, 0), verifyTOTP(t, g, id, "123456", 0)}; !reflect.DeepEqual(got, []bool{false, true}) {
		t.Fatalf("ok = %v, want false for a wrong code, true for the right one", got)
	}
	s, err := g.client.GetMFAChallenge(ctx, connect.NewRequest(&gatewayv1.GetMFAChallengeRequest{ChallengeId: id}))
	if err != nil || s.Msg.GetStatus() != "confirmed" {
		t.Fatalf("status %v, %v; want confirmed", s, err)
	}
	fm.with(func(m *fakeMFA) {
		if m.verifies[0] != [3]uuid.UUID{b.org, peer.InstanceID, uuid.MustParse(id)} {
			t.Fatalf("VerifyTOTP(%v), want the caller's org and instance", m.verifies[0])
		}
	})
}

// A resource outside the caller's pool is refused; malformed fields are
// InvalidArgument; nothing reaches MFA, and without MFA the RPCs are unimplemented.
func TestMFAChallengeRPCValidation(t *testing.T) {
	b, _, fm := newMFABench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	user, res := uuid.New(), uuid.New()
	_, err := g.client.CreateMFAChallenge(ctx, connect.NewRequest(challengeReq(user, res)))
	wantCode(t, err, connect.CodePermissionDenied, "")
	b.f.with(func(f *fake) { f.owned = true })
	for name, edit := range map[string]func(r *gatewayv1.CreateMFAChallengeRequest){
		"empty ssh session": func(r *gatewayv1.CreateMFAChallengeRequest) { r.SshSessionId = nil },
		"long ssh session":  func(r *gatewayv1.CreateMFAChallengeRequest) { r.SshSessionId = make([]byte, 65) },
		"strength":          func(r *gatewayv1.CreateMFAChallengeRequest) { r.Strength = "x" },
		"user id":           func(r *gatewayv1.CreateMFAChallengeRequest) { r.UserId = "x" },
		"resource id":       func(r *gatewayv1.CreateMFAChallengeRequest) { r.ResourceId = "x" },
		"client ip":         func(r *gatewayv1.CreateMFAChallengeRequest) { r.ClientIp = "x" },
	} {
		r := challengeReq(user, res)
		edit(r)
		if _, err := g.client.CreateMFAChallenge(ctx, connect.NewRequest(r)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("%s: err = %v, want InvalidArgument", name, err)
		}
	}
	verifyTOTP(t, g, "x", "123456", connect.CodeInvalidArgument)
	fm.with(func(m *fakeMFA) {
		if len(m.creates)+len(m.verifies) != 0 {
			t.Fatalf("MFA calls %v %v, want none", m.creates, m.verifies)
		}
	})

	h, gctx, _ := handlerFor(Deps{})
	_, err = h.CreateMFAChallenge(gctx, connect.NewRequest(challengeReq(user, res)))
	wantCode(t, err, connect.CodeUnimplemented, "")
	_, err = h.GetMFAChallenge(gctx, connect.NewRequest(&gatewayv1.GetMFAChallengeRequest{ChallengeId: uuid.NewString()}))
	wantCode(t, err, connect.CodeUnimplemented, "")
}

// The gateway may name only key and certificate bindings; web and CLI
// bindings are the server's own.
func TestMFAChallengeRPCBindingClass(t *testing.T) {
	b, _, _ := newMFABench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	b.f.with(func(f *fake) { f.owned = true })
	sha := strings.Repeat("a", 42) + "/"
	for binding, ok := range map[string]bool{
		"key:SHA256:" + sha:                      true,
		"cert:SHA256:" + sha:                     true,
		"web:" + uuid.NewString():                false,
		"cli:" + uuid.NewString():                false,
		"key:SHA256:short":                       false,
		"key:SHA256:" + sha + "A":                false,
		"key:MD5:" + sha:                         false,
		"key:SHA256:" + strings.Repeat("-", 43):  false,
		"":                                       false,
		"Key:SHA256:" + strings.Repeat("A", 43):  false,
		" key:SHA256:" + strings.Repeat("A", 43): false,
	} {
		r := challengeReq(uuid.New(), uuid.New())
		r.Binding = binding
		_, err := g.client.CreateMFAChallenge(ctx, connect.NewRequest(r))
		if ok && err != nil || !ok && connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("binding %q: err = %v, want accepted %v", binding, err, ok)
		}
	}
}

// AuthenticateUser answers the age and method of the last MFA of the
// request's binding; a login, an agent and a ticket without a cookie
// session have none and ask for no age.
func TestAuthenticateUserMFAAge(t *testing.T) {
	b, rt, fm := newMFABench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	alice := User{ID: uuid.New(), Login: "alice"}
	agent := User{ID: alice.ID, Login: "alice", Agent: &AgentRef{AgentID: uuid.New(), RunID: uuid.New()}}
	userSession, ticket, bare := uuid.New(), uuid.New(), uuid.New()
	cert := func(session uuid.UUID) *gatewayv1.AuthenticateUserRequest {
		return &gatewayv1.AuthenticateUserRequest{By: &gatewayv1.AuthenticateUserRequest_Cert{Cert: &gatewayv1.CertIdentity{
			UserId: alice.ID.String(), CliSessionId: session.String(), Login: "alice", CertFingerprint: "SHA256:cert"}}}
	}
	person, robot := uuid.New(), uuid.New()
	certKey := func(session uuid.UUID) string {
		return "cert:" + b.org.String() + " " + alice.ID.String() + " " + session.String() + " alice"
	}
	rt.set(func(r *realtimeFake) {
		r.user = alice
		r.tickets[ticket] = WebTicket{Mode: "web", Resource: "db", UserSessionID: &userSession}
		r.tickets[bare] = WebTicket{Mode: "web", Resource: "db"}
	})
	b.f.with(func(f *fake) {
		f.users["fp:"+keyBinding[4:]], f.users["login:alice"], f.users[certKey(person)], f.users[certKey(robot)] = alice, alice, alice, agent
	})
	fm.with(func(m *fakeMFA) { m.age, m.method = 42, "webauthn" })
	for name, c := range map[string]struct {
		req     *gatewayv1.AuthenticateUserRequest
		binding string // empty: no MFA and no Age call
	}{
		"key":                    {&gatewayv1.AuthenticateUserRequest{By: &gatewayv1.AuthenticateUserRequest_FingerprintSha256{FingerprintSha256: keyBinding[4:]}}, keyBinding},
		"cert":                   {cert(person), "cert:SHA256:cert"},
		"web":                    {&gatewayv1.AuthenticateUserRequest{By: &gatewayv1.AuthenticateUserRequest_WebTicket{WebTicket: ticket.String()}}, "web:" + userSession.String()},
		"login":                  {&gatewayv1.AuthenticateUserRequest{By: &gatewayv1.AuthenticateUserRequest_Login{Login: "alice"}}, ""},
		"agent":                  {cert(robot), ""},
		"ticket without session": {&gatewayv1.AuthenticateUserRequest{By: &gatewayv1.AuthenticateUserRequest_WebTicket{WebTicket: bare.String()}}, ""},
	} {
		var ages []string
		fm.with(func(m *fakeMFA) { m.ages = nil })
		c.req.ClientIp = "10.0.0.1"
		resp, err := g.client.AuthenticateUser(ctx, connect.NewRequest(c.req))
		fm.with(func(m *fakeMFA) { ages = m.ages })
		want := []any{int64(42), "webauthn", []string{c.binding}}
		if c.binding == "" {
			want = []any{int64(math.MaxInt64), "", []string(nil)}
		}
		if got := []any{resp.Msg.GetMfaAgeSeconds(), resp.Msg.GetMfaMethod(), ages}; err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: mfa and Age bindings %v, %v; want %v", name, got, err, want)
		}
	}
}

// nextMFA waits for the next mfa_confirmed, skipping other messages; nil when none came within d.
func nextMFA(t *testing.T, msgs <-chan *gatewayv1.ConnectResponse, errc <-chan error, d time.Duration) *gatewayv1.MFAConfirmed {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case m := <-msgs:
			if c := m.GetMfaConfirmed(); c != nil {
				return c
			}
		case err := <-errc:
			t.Fatalf("stream closed: %v", err)
		case <-deadline:
			return nil
		}
	}
}

// mfa_confirmed reaches only the owning instance; a malformed payload sends
// nothing and is logged only by its length.
func TestMFAConfirmedDelivered(t *testing.T) {
	var logs syncBuffer
	b, _, _ := newMFABench(t, &logs)
	pool := uuid.New()
	g, peer := b.instance(t, b.org, pool), b.instance(t, b.org, pool)
	msgs, errc := rawConn(t, g)
	firstSnapshot(t, msgs)
	peerMsgs, peerErr := rawConn(t, peer)
	firstSnapshot(t, peerMsgs)

	id := uuid.New()
	b.hub.MFAConfirmed(uuid.NewString() + ":" + uuid.NewString())
	b.hub.MFAConfirmed("garbage")
	b.hub.MFAConfirmed(g.InstanceID.String() + ":not-a-challenge")
	b.hub.MFAConfirmed(g.InstanceID.String() + ":" + id.String())
	if got := nextMFA(t, msgs, errc, time.Second); got.GetChallengeId() != id.String() {
		t.Fatalf("got %v, want mfa_confirmed %s", got, id)
	}
	if extra := nextMFA(t, msgs, errc, 300*time.Millisecond); extra != nil {
		t.Fatalf("a foreign or malformed payload sent %v", extra)
	}
	if extra := nextMFA(t, peerMsgs, peerErr, 100*time.Millisecond); extra != nil {
		t.Fatalf("another instance got %v", extra)
	}
	out := string(logs.bytes())
	if !strings.Contains(out, "payload_len") || strings.Contains(out, "garbage") || strings.Contains(out, "not-a-challenge") ||
		strings.Contains(out, id.String()) {
		t.Fatalf("malformed payload log must carry only payload_len and the instance id:\n%s", out)
	}
}

// The listener never blocks on a busy stream; a full queue closes the
// stream with overflow, and the reconnected gateway gets the resend.
func TestMFAConfirmedOverflowClosesStream(t *testing.T) {
	b, rt, _ := newMFABench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	hold := make(chan struct{})
	rt.set(func(r *realtimeFake) { r.pendHold = hold }) // the stream stops at its first resend
	msgs, errc := rawConn(t, g)
	firstSnapshot(t, msgs)

	payload := g.InstanceID.String() + ":" + uuid.NewString()
	start := time.Now()
	for range 1000 {
		b.hub.MFAConfirmed(payload)
	}
	if d := time.Since(start); d >= 100*time.Millisecond {
		t.Fatalf("1000 MFAConfirmed took %v with the stream stuck", d)
	}
	close(hold)
	select {
	case err := <-errc:
		wantCode(t, err, connect.CodeUnavailable, "overflow")
	case <-time.After(2 * time.Second):
		t.Fatal("stream not closed after the control queue overflowed")
	}
}

// Confirmed challenges of the instance are resent after the first snapshot.
func TestResendConfirmedOnConnect(t *testing.T) {
	b, _, fm := newMFABench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	c1, c2 := uuid.New(), uuid.New()
	fm.with(func(m *fakeMFA) { m.confirmed = []uuid.UUID{c1, c2} })
	msgs, errc := rawConn(t, g)
	firstSnapshot(t, msgs)
	for _, want := range []uuid.UUID{c1, c2} {
		if got := nextMFA(t, msgs, errc, time.Second); got.GetChallengeId() != want.String() {
			t.Fatalf("got %v, want mfa_confirmed %s", got, want)
		}
	}
}
