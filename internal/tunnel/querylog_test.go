package tunnel

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

func dbStart(dbName string) *gatewayv1.StartSessionRequest {
	return &gatewayv1.StartSessionRequest{UserId: uuid.NewString(), ResourceId: uuid.NewString(), ClientIp: "10.1.2.3",
		Kind: "stream", Target: "db:5432", DbName: dbName}
}

// StartSession passes the database name and begins the recording of the new session.
func TestTunnelStartSessionDBName(t *testing.T) {
	f := newFake()
	f.owned = true
	ingest := &beginIngest{fakeIngest: &f.ingest}
	d := f.deps()
	d.Recordings = ingest
	h, gctx, g := handlerFor(d)
	resp, err := h.StartSession(gctx, connect.NewRequest(dbStart("app")))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.starts) != 1 || f.starts[0].DBName != "app" {
		t.Fatalf("starts %+v, want one with DBName app", f.starts)
	}
	if resp.Msg.GetSessionId() != f.sessionID.String() {
		t.Fatalf("session id %q", resp.Msg.GetSessionId())
	}
	if want := [][2]uuid.UUID{{g.OrgID, f.sessionID}}; len(ingest.begins) != 1 || ingest.begins[0] != want[0] {
		t.Fatalf("begins %v, want %v", ingest.begins, want)
	}
	if len(f.ends) != 0 {
		t.Fatalf("ends %+v, want none", f.ends)
	}
}

// A recording that cannot begin ends the new session with "error" and answers
// Unavailable: no session runs unrecorded.
func TestTunnelBeginFailureEndsSession(t *testing.T) {
	f := newFake()
	f.owned = true
	d := f.deps()
	d.Recordings = &beginIngest{fakeIngest: &f.ingest, err: errors.New("database down")}
	h, gctx, g := handlerFor(d)
	_, err := h.StartSession(gctx, connect.NewRequest(dbStart("app")))
	wantCode(t, err, connect.CodeUnavailable, "")
	if len(f.ends) != 1 {
		t.Fatalf("ends %+v, want one", f.ends)
	}
	if e := f.ends[0]; e.id != f.sessionID || e.reason != "error" || e.orgID != g.OrgID || e.instanceID != g.InstanceID {
		t.Fatalf("end %+v, want session %v ended with error by its instance", e, f.sessionID)
	}
}

// EndSession passes the query count, also as the only result of a stream session.
func TestTunnelEndSessionQueries(t *testing.T) {
	s := &resultSessions{fake: newFake()}
	h, gctx, _ := handlerFor(Deps{Sessions: s})
	id := uuid.NewString()
	for _, req := range []*gatewayv1.EndSessionRequest{
		{SessionId: id, Reason: "closed", Queries: 5},
		{SessionId: id, Reason: "closed", Command: "ls", Queries: 2},
		{SessionId: id, Reason: "closed"},
	} {
		if _, err := h.EndSession(gctx, connect.NewRequest(req)); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.results) != 3 || s.results[0] == nil || s.results[0].Queries != 5 ||
		s.results[1] == nil || s.results[1].Queries != 2 || s.results[1].Command != "ls" || s.results[2] != nil {
		t.Fatalf("End results %+v, want queries 5, then 2 with ls, then none", s.results)
	}
}
