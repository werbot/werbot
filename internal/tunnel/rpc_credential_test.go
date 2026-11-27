package tunnel

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// credBench is a bench whose credentials are a credFake.
func credBench(t *testing.T) (*bench, *credFake, *gw) {
	t.Helper()
	c := &credFake{secret: Secret{Kind: "password", Value: []byte("pw-marker")}, version: 42}
	b := newBenchTLS(t, nil, nil, func(d *Deps) { c.fake = d.Credentials.(*fake); d.Credentials = c })
	return b, c, b.instance(t, b.org, uuid.New())
}

// sameCaller reports whether a is the instance b: pool, instance and organization.
func sameCaller(a, b Gateway) bool {
	return a.ID == b.ID && a.InstanceID == b.InstanceID && a.OrgID == b.OrgID
}

// sameWrite reports whether a and b check the same write of the same caller.
func sameWrite(a, b credCall) bool {
	return sameCaller(a.g, b.g) && a.id == b.id && a.approval == b.approval && bytes.Equal(a.sha, b.sha)
}

// writeCall asks g for the write credential.
func writeCall(g *gw, session, approval string, sum []byte) (*gatewayv1.GetWriteCredentialResponse, error) {
	res, err := g.client.GetWriteCredential(ctx, connect.NewRequest(&gatewayv1.GetWriteCredentialRequest{SessionId: session, ApprovalId: approval, OpSha256: sum}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// GetWriteCredential asks the server's binding check with the caller from
// the certificate and answers its secret; a refusal keeps its code; invalid
// ids and a hash of another length are InvalidArgument before any check.
func TestGetWriteCredential(t *testing.T) {
	_, c, g := credBench(t)
	session, approval, sha := uuid.New(), uuid.New(), sha256.Sum256([]byte("UPDATE t SET a = 1"))
	res, err := writeCall(g, session.String(), approval.String(), sha[:])
	if err != nil {
		t.Fatal(err)
	}
	if res.GetCredential().GetKind() != "password" || !bytes.Equal(res.GetCredential().GetValue(), []byte("pw-marker")) {
		t.Fatal("credential not returned")
	}
	if len(c.writes) != 1 || !sameWrite(c.writes[0], credCall{g: g.Gateway, id: session, approval: approval, sha: sha[:]}) {
		t.Fatalf("binding check called with %+v", c.writes)
	}

	for _, bad := range [][3]any{{"x", approval.String(), sha[:]}, {session.String(), "x", sha[:]}, {session.String(), approval.String(), sha[:31]}} {
		_, err := writeCall(g, bad[0].(string), bad[1].(string), bad[2].([]byte))
		wantCode(t, err, connect.CodeInvalidArgument, "")
	}
	c.err = connect.NewError(connect.CodePermissionDenied, fmt.Errorf("write not approved"))
	_, err = writeCall(g, session.String(), approval.String(), sha[:])
	wantCode(t, err, connect.CodePermissionDenied, "write not approved")
	if len(c.writes) != 2 {
		t.Fatalf("%d binding checks, want 2", len(c.writes))
	}
}

// GetProbeCredential serves only the agent_readonly slot (Ruling C15): the
// primary slot and an unknown one are InvalidArgument without a lookup; the
// lookup gets the snapshot's configuration version (Ruling C16); the answer
// carries the check version.
func TestGetProbeCredentialSlot(t *testing.T) {
	_, c, g := credBench(t)
	resource := uuid.New()
	call := func(slot string) (*gatewayv1.GetProbeCredentialResponse, error) {
		res, err := g.client.GetProbeCredential(ctx, connect.NewRequest(&gatewayv1.GetProbeCredentialRequest{ResourceId: resource.String(), Slot: slot, ConfigVersion: 9}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	for _, slot := range []string{"primary", "x", ""} {
		_, err := call(slot)
		wantCode(t, err, connect.CodeInvalidArgument, "")
	}
	if len(c.probes) != 0 {
		t.Fatal("a slot other than agent_readonly was looked up")
	}
	res, err := call("agent_readonly")
	if err != nil {
		t.Fatal(err)
	}
	if res.GetVersion() != 42 || !bytes.Equal(res.GetCredential().GetValue(), []byte("pw-marker")) {
		t.Fatal("credential or version not returned")
	}
	if len(c.probes) != 1 || !sameCaller(c.probes[0].g, g.Gateway) || c.probes[0].id != resource || c.probes[0].config != 9 {
		t.Fatalf("probe lookup called with %+v", c.probes)
	}
	c.err = fmt.Errorf("resource not found: %w", apperr.ErrNotFound)
	_, err = call("agent_readonly")
	wantCode(t, err, connect.CodeNotFound, "")
}

// StartSession asks for the agent slot only for an agent's session.
func TestStartSessionRevealsAgentSlot(t *testing.T) {
	b, c, g := credBench(t)
	b.f.with(func(f *fake) { f.owned, f.ownedType = true, "postgres" })
	for _, run := range []string{uuid.NewString(), ""} {
		_, err := g.client.StartSession(ctx, connect.NewRequest(&gatewayv1.StartSessionRequest{UserId: uuid.NewString(),
			ResourceId: uuid.NewString(), ClientIp: "10.1.2.3", Kind: "stream", Target: "db:5432", AgentRunId: run}))
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(c.reveals) != 2 || !c.reveals[0].agent || c.reveals[1].agent || c.reveals[0].typ != "postgres" {
		t.Fatalf("reveals %+v, want agent then person", c.reveals)
	}
}

// AuthenticateUser sends an agent run's database limits; statement_timeout_ms is milliseconds.
func TestAuthenticateUserAgentLimits(t *testing.T) {
	u := User{ID: uuid.New(), Agent: &AgentRef{AgentID: uuid.New(), RunID: uuid.New(),
		Limits: AgentLimits{MaxRows: 50, MaxResultBytes: 1 << 20, StatementTimeout: 30 * time.Second, MaxQueries: 1000}}}
	l := agentInfo(u).GetLimits()
	if l.GetMaxRows() != 50 || l.GetMaxResultBytes() != 1048576 || l.GetStatementTimeoutMs() != 30000 || l.GetMaxQueries() != 1000 {
		t.Fatalf("limits %v", l)
	}
	if agentInfo(User{ID: uuid.New()}) != nil {
		t.Fatal("a person has no agent info")
	}
}

// A heartbeat's probe result carries the agent_readonly check unchanged.
func TestHeartbeatHealthAgentReadonly(t *testing.T) {
	id := uuid.New()
	got, dropped := heartbeatHealth([]*gatewayv1.ResourceHealth{{ResourceId: id.String(), Ok: true, CheckedAt: timestamppb.Now(),
		AgentReadonlyStatus: "has_write", AgentReadonlyDetail: "INSERT on db.public.t", AgentReadonlyVersion: 7}})
	if dropped != 0 || len(got) != 1 {
		t.Fatalf("got %v dropped %d", got, dropped)
	}
	if h := got[0]; h.AgentReadonlyStatus != "has_write" || h.AgentReadonlyDetail != "INSERT on db.public.t" || h.AgentReadonlyVersion != 7 {
		t.Fatalf("health %+v", h)
	}
}
