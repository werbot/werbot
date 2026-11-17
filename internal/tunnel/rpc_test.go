package tunnel

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

var ctx = context.Background()

func TestRejectsUnknownInstance(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	b.f.with(func(f *fake) { f.authErr[g.InstanceID] = apperr.ErrUnauthenticated })

	_, err := g.client.AuthenticateUser(ctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
		By: &gatewayv1.AuthenticateUserRequest_Login{Login: "alice"}, ClientIp: "10.0.0.1"}))
	wantCode(t, err, connect.CodeUnauthenticated, "")
	b.f.with(func(f *fake) {
		if len(f.auths) != 1 || f.auths[0] != (authCall{g.ID, g.InstanceID, g.serial}) {
			t.Fatalf("Authenticate calls = %+v, want gw/instance/serial from the certificate", f.auths)
		}
		if len(f.userOrgs) != 0 {
			t.Fatal("handler ran for a rejected instance")
		}
	})

	// A request without a TLS peer certificate never reaches the handler.
	rec := httptest.NewRecorder()
	New(b.f.deps(), NewHub()).mtls(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no TLS: status %d, want 401", rec.Code)
	}
}

func TestAuthenticateUser(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	alice := User{ID: uuid.New(), Login: "alice", TeamIDs: []uuid.UUID{uuid.New(), uuid.New()}}
	b.f.with(func(f *fake) { f.users["fp:SHA256:abc"], f.users["login:alice"] = alice, alice })

	for name, req := range map[string]*gatewayv1.AuthenticateUserRequest{
		"fingerprint": {By: &gatewayv1.AuthenticateUserRequest_FingerprintSha256{FingerprintSha256: "SHA256:abc"}, ClientIp: "203.0.113.7"},
		"login":       {By: &gatewayv1.AuthenticateUserRequest_Login{Login: "alice"}, ClientIp: "203.0.113.7"},
	} {
		resp, err := g.client.AuthenticateUser(ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		m := resp.Msg
		if m.GetUserId() != alice.ID.String() || m.GetLogin() != "alice" || m.GetCountry() != "NL" ||
			!slices.Equal(m.GetTeamIds(), []string{alice.TeamIDs[0].String(), alice.TeamIDs[1].String()}) {
			t.Fatalf("%s: response %+v", name, m)
		}
	}
	b.f.with(func(f *fake) {
		if len(f.userOrgs) != 2 || f.userOrgs[0] != b.org || f.userOrgs[1] != b.org {
			t.Fatalf("lookups used orgs %v, want the instance org %v", f.userOrgs, b.org)
		}
	})

	for _, ip := range []string{"", "not-an-ip"} {
		_, err := g.client.AuthenticateUser(ctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
			By: &gatewayv1.AuthenticateUserRequest_Login{Login: "alice"}, ClientIp: ip}))
		wantCode(t, err, connect.CodeInvalidArgument, "")
	}
	_, err := g.client.AuthenticateUser(ctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
		By: &gatewayv1.AuthenticateUserRequest_Login{Login: "bob"}, ClientIp: "10.0.0.1"}))
	wantCode(t, err, connect.CodeNotFound, "")
}

func TestStartSessionOnlyOwnPool(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	user, res := uuid.New(), uuid.New()
	req := &gatewayv1.StartSessionRequest{UserId: user.String(), ResourceId: res.String(), ClientIp: "10.1.2.3",
		Kind: "stream", Target: "db:5432"}

	_, err := g.client.StartSession(ctx, connect.NewRequest(req))
	wantCode(t, err, connect.CodePermissionDenied, "")
	b.f.with(func(f *fake) {
		if f.ownedCalls[0] != [3]uuid.UUID{b.org, g.ID, res} {
			t.Fatalf("OwnedBy(%v), want org, pool, resource", f.ownedCalls[0])
		}
		if f.reveals != 0 || len(f.starts) != 0 {
			t.Fatal("Reveal/Start called for a foreign resource")
		}
	})

	// No credential: no session row is opened.
	b.f.with(func(f *fake) { f.owned, f.revealErr = true, apperr.ErrNotFound })
	_, err = g.client.StartSession(ctx, connect.NewRequest(req))
	wantCode(t, err, connect.CodeNotFound, "")
	b.f.with(func(f *fake) {
		if len(f.starts) != 0 {
			t.Fatal("Start called although Reveal failed")
		}
	})

	b.f.with(func(f *fake) { f.revealErr, f.secret = nil, Secret{Kind: "password", Value: []byte("s3cret")} })
	resp, err := g.client.StartSession(ctx, connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.GetSessionId() != b.f.sessionID.String() || resp.Msg.GetCredential().GetKind() != "password" ||
		!bytes.Equal(resp.Msg.GetCredential().GetValue(), []byte("s3cret")) {
		t.Fatalf("response %+v", resp.Msg)
	}
	want := SessionStart{OrgID: b.org, GatewayID: g.ID, InstanceID: g.InstanceID, UserID: user, ResourceID: res,
		ClientIP: netip.MustParseAddr("10.1.2.3"), Kind: "stream", Target: "db:5432"}
	b.f.with(func(f *fake) {
		if len(f.starts) != 1 || f.starts[0] != want {
			t.Fatalf("Start(%+v), want %+v", f.starts, want)
		}
	})

	for _, bad := range []*gatewayv1.StartSessionRequest{
		{UserId: "x", ResourceId: res.String(), ClientIp: "10.1.2.3", Kind: "terminal"},
		{UserId: user.String(), ResourceId: "x", ClientIp: "10.1.2.3", Kind: "terminal"},
		{UserId: user.String(), ResourceId: res.String(), ClientIp: "x", Kind: "terminal"},
	} {
		_, err := g.client.StartSession(ctx, connect.NewRequest(bad))
		wantCode(t, err, connect.CodeInvalidArgument, "")
	}
}

func TestStartSessionKindRequired(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	b.f.with(func(f *fake) { f.owned = true })
	user, res := uuid.NewString(), uuid.NewString()
	for _, bad := range []*gatewayv1.StartSessionRequest{
		{UserId: user, ResourceId: res, ClientIp: "10.1.2.3"},
		{UserId: user, ResourceId: res, ClientIp: "10.1.2.3", Kind: "shell"},
		{UserId: user, ResourceId: res, ClientIp: "10.1.2.3", Kind: "stream", Target: strings.Repeat("h", 301)},
	} {
		_, err := g.client.StartSession(ctx, connect.NewRequest(bad))
		wantCode(t, err, connect.CodeInvalidArgument, "")
	}
	b.f.with(func(f *fake) {
		if len(f.starts) != 0 {
			t.Fatalf("Start called: %+v", f.starts)
		}
	})
	for _, kind := range []string{"terminal", "exec", "stream", "datagram"} {
		_, err := g.client.StartSession(ctx, connect.NewRequest(&gatewayv1.StartSessionRequest{
			UserId: user, ResourceId: res, ClientIp: "10.1.2.3", Kind: kind}))
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
}

func TestEndSessionCountersAndReasons(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	id := uuid.New()
	for _, reason := range []string{"idle_timeout", "max_duration", "access_revoked"} {
		_, err := g.client.EndSession(ctx, connect.NewRequest(&gatewayv1.EndSessionRequest{SessionId: id.String(), Reason: reason,
			Counters: &gatewayv1.SessionCounters{SessionId: id.String(), BytesUp: 7, BytesDown: 8, DatagramsUp: 9, DatagramsDown: 10}}))
		if err != nil {
			t.Fatalf("%s: %v", reason, err)
		}
	}
	_, err := g.client.EndSession(ctx, connect.NewRequest(&gatewayv1.EndSessionRequest{SessionId: id.String(), Reason: "gateway_lost"}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
	_, err = g.client.EndSession(ctx, connect.NewRequest(&gatewayv1.EndSessionRequest{SessionId: id.String(), Reason: "closed",
		Counters: &gatewayv1.SessionCounters{SessionId: uuid.NewString(), BytesUp: 1}}))
	wantCode(t, err, connect.CodeInvalidArgument, "invalid counters.session_id")
	b.f.with(func(f *fake) {
		want := Counters{SessionID: id, BytesUp: 7, BytesDown: 8, DatagramsUp: 9, DatagramsDown: 10}
		if len(f.ends) != 3 || f.ends[2].reason != "access_revoked" || f.ends[2].c == nil || *f.ends[2].c != want {
			t.Fatalf("End(%+v), want counters %+v", f.ends, want)
		}
	})
}

func TestEndSessionScopedToInstance(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	id := uuid.New()
	if _, err := g.client.EndSession(ctx, connect.NewRequest(&gatewayv1.EndSessionRequest{SessionId: id.String(), Reason: "closed"})); err != nil {
		t.Fatal(err)
	}
	b.f.with(func(f *fake) {
		want := endCall{orgID: b.org, instanceID: g.InstanceID, id: id, reason: "closed"}
		if len(f.ends) != 1 || f.ends[0].orgID != want.orgID || f.ends[0].instanceID != want.instanceID ||
			f.ends[0].id != want.id || f.ends[0].reason != want.reason || f.ends[0].c != nil {
			t.Fatalf("End(%+v), want %+v", f.ends, want)
		}
		f.endErr = apperr.ErrNotFound
	})
	_, err := g.client.EndSession(ctx, connect.NewRequest(&gatewayv1.EndSessionRequest{SessionId: id.String(), Reason: "closed"}))
	wantCode(t, err, connect.CodeNotFound, "")
	_, err = g.client.EndSession(ctx, connect.NewRequest(&gatewayv1.EndSessionRequest{SessionId: "x"}))
	wantCode(t, err, connect.CodeInvalidArgument, "")
}

func TestPinHostKey(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	key, err := gossh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	res := uuid.New().String()
	pin := func(alg string, raw []byte) error {
		_, err := g.client.PinHostKey(ctx, connect.NewRequest(&gatewayv1.PinHostKeyRequest{ResourceId: res, PublicKey: raw, Alg: alg}))
		return err
	}

	wantCode(t, pin(key.Type(), key.Marshal()), connect.CodePermissionDenied, "")
	b.f.with(func(f *fake) { f.owned = true })
	wantCode(t, pin("ssh-rsa", key.Marshal()), connect.CodeInvalidArgument, "")
	wantCode(t, pin(key.Type(), []byte("garbage")), connect.CodeInvalidArgument, "")
	if err := pin(key.Type(), key.Marshal()); err != nil {
		t.Fatal(err)
	}
	b.f.with(func(f *fake) {
		if len(f.pinned) != 1 || !bytes.Equal(f.pinned[0].Marshal(), key.Marshal()) {
			t.Fatalf("pinned %v", f.pinned)
		}
		f.pinErr = apperr.ErrFailedPrecondition
	})
	wantCode(t, pin(key.Type(), key.Marshal()), connect.CodeFailedPrecondition, "")
}

func TestRenewCertificate(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	resp, err := g.client.RenewCertificate(ctx, connect.NewRequest(&gatewayv1.RenewCertificateRequest{CsrDer: []byte("CSR")}))
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Msg.GetCertPem()) != "CERT" {
		t.Fatalf("cert_pem %q", resp.Msg.GetCertPem())
	}
	b.f.with(func(f *fake) {
		if len(f.renewed) != 1 || f.renewed[0] != g.InstanceID || string(f.renewCSR) != "CSR" {
			t.Fatalf("Renew(%v, %q), want the certificate instance", f.renewed, f.renewCSR)
		}
	})
}
