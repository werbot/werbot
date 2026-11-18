package tunnel

import (
	"context"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

type authCall struct {
	gatewayID, instanceID uuid.UUID
	serial                string
}

type hbCall struct {
	instanceID uuid.UUID
	version    string
	connectors []string
}

type endCall struct {
	orgID, instanceID, id uuid.UUID
	active                []uuid.UUID
	reason                string
	c                     *Counters
}

// fake implements every dependency interface and records the calls.
type fake struct {
	mu        sync.Mutex
	instances map[uuid.UUID]Gateway // instance id -> identity
	authErr   map[uuid.UUID]error   // instance id -> forced Authenticate error
	auths     []authCall
	hbs       []hbCall
	hbErr     error

	renewed  []uuid.UUID
	renewCSR []byte

	users      map[string]User // "fp:<fp>" / "login:<login>"
	userOrgs   []uuid.UUID
	owned      bool
	ownedType  string   // the resource type OwnedBy reports
	revealed   []string // the resource type of every Reveal
	ownedCalls [][3]uuid.UUID
	pinned     []gossh.PublicKey
	pinErr     error

	secret    Secret
	revealErr error
	reveals   int
	starts    []SessionStart
	sessionID uuid.UUID
	ends      []endCall
	endErr    error
	missing   []endCall

	builds    map[[2]uuid.UUID]int // (org, pool) -> count
	buildFail int                  // fail the next N builds
	snapshot  func(orgID, gatewayID uuid.UUID) *gatewayv1.Snapshot

	buildDelay  atomic.Int64 // nanoseconds every Build sleeps outside the lock
	inflight    atomic.Int32 // Build calls running now
	maxInflight atomic.Int32 // highest inflight seen

	ingest fakeIngest
	beat   beatRecord
}

// recCall is one IngestChunk (seq, hash, gz) or FinishRecording (chunks, bytes, hash) call.
type recCall struct {
	orgID, instanceID, sessionID uuid.UUID
	n                            int
	bytes                        int64
	hash, gz                     []byte
}

// fakeIngest implements RecordingIngest and returns err from every call.
type fakeIngest struct {
	mu       sync.Mutex
	err      error
	chunks   []recCall
	finishes []recCall
}

func (r *fakeIngest) IngestChunk(_ context.Context, orgID, instanceID, sessionID uuid.UUID, seq int, hash, gz []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chunks = append(r.chunks, recCall{orgID: orgID, instanceID: instanceID, sessionID: sessionID, n: seq, hash: hash, gz: gz})
	return r.err
}

func (r *fakeIngest) FinishRecording(_ context.Context, orgID, instanceID, sessionID uuid.UUID, chunks int, bytes int64, finalHash []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finishes = append(r.finishes, recCall{orgID: orgID, instanceID: instanceID, sessionID: sessionID, n: chunks, bytes: bytes, hash: finalHash})
	return r.err
}

// with runs fn under the fake's lock to read or change its state.
func (r *fakeIngest) with(fn func(r *fakeIngest)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fn(r)
}

func newFake() *fake {
	return &fake{instances: map[uuid.UUID]Gateway{}, authErr: map[uuid.UUID]error{}, users: map[string]User{},
		builds: map[[2]uuid.UUID]int{}, sessionID: uuid.New()}
}

func (f *fake) Authenticate(_ context.Context, gatewayID, instanceID uuid.UUID, serial string) (Gateway, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.auths = append(f.auths, authCall{gatewayID, instanceID, serial})
	if err := f.authErr[instanceID]; err != nil {
		return Gateway{}, err
	}
	g, ok := f.instances[instanceID]
	if !ok || g.ID != gatewayID {
		return Gateway{}, apperr.ErrUnauthenticated
	}
	g.CertSerial = serial // every accepted serial is the current one
	return g, nil
}

func (f *fake) Heartbeat(_ context.Context, instanceID uuid.UUID, version string, connectors []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hbs = append(f.hbs, hbCall{instanceID, version, connectors})
	return f.hbErr
}

func (f *fake) Renew(_ context.Context, instanceID uuid.UUID, csrDER []byte) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.renewed = append(f.renewed, instanceID)
	f.renewCSR = csrDER
	return []byte("CERT"), nil
}

func (f *fake) lookup(orgID uuid.UUID, key string) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.userOrgs = append(f.userOrgs, orgID)
	u, ok := f.users[key]
	if !ok {
		return User{}, apperr.ErrNotFound
	}
	return u, nil
}

func (f *fake) ByFingerprint(_ context.Context, orgID uuid.UUID, fp string) (User, error) {
	return f.lookup(orgID, "fp:"+fp)
}

func (f *fake) ByLogin(_ context.Context, orgID uuid.UUID, login string) (User, error) {
	return f.lookup(orgID, "login:"+login)
}

func (f *fake) OwnedBy(_ context.Context, orgID, gatewayID, resourceID uuid.UUID) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ownedCalls = append(f.ownedCalls, [3]uuid.UUID{orgID, gatewayID, resourceID})
	if !f.owned {
		return "", false, nil
	}
	return f.ownedType, true, nil
}

func (f *fake) PinHostKey(_ context.Context, _, _ uuid.UUID, key gossh.PublicKey) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pinErr != nil {
		return f.pinErr
	}
	f.pinned = append(f.pinned, key)
	return nil
}

func (f *fake) Reveal(_ context.Context, _, _ uuid.UUID, resourceType string) (Secret, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reveals++
	f.revealed = append(f.revealed, resourceType)
	return f.secret, f.revealErr
}

func (f *fake) Start(_ context.Context, s SessionStart) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts = append(f.starts, s)
	return f.sessionID, nil
}

func (f *fake) End(_ context.Context, orgID, instanceID, id uuid.UUID, reason string, c *Counters) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ends = append(f.ends, endCall{orgID: orgID, instanceID: instanceID, id: id, reason: reason, c: c})
	return f.endErr
}

func (f *fake) EndMissing(_ context.Context, orgID, instanceID uuid.UUID, active []uuid.UUID, reason string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.missing = append(f.missing, endCall{orgID: orgID, instanceID: instanceID, active: active, reason: reason})
	return 0, nil
}

func (f *fake) Build(_ context.Context, orgID, gatewayID uuid.UUID) (*gatewayv1.Snapshot, error) {
	n := f.inflight.Add(1)
	defer f.inflight.Add(-1)
	for m := f.maxInflight.Load(); n > m; m = f.maxInflight.Load() {
		if f.maxInflight.CompareAndSwap(m, n) {
			break
		}
	}
	time.Sleep(time.Duration(f.buildDelay.Load()))
	f.mu.Lock()
	defer f.mu.Unlock()
	f.builds[[2]uuid.UUID{orgID, gatewayID}]++
	if f.buildFail > 0 {
		f.buildFail--
		return nil, apperr.ErrUnavailable
	}
	if f.snapshot != nil {
		return f.snapshot(orgID, gatewayID), nil
	}
	return &gatewayv1.Snapshot{CedarPolicies: "permit(principal, action, resource);", Blocklist: []string{gatewayID.String()}}, nil
}

func (f *fake) buildCount(orgID, gatewayID uuid.UUID) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.builds[[2]uuid.UUID{orgID, gatewayID}]
}

// with runs fn under the fake's lock to read or change its state.
func (f *fake) with(fn func(f *fake)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func (f *fake) deps() Deps {
	return Deps{Gateways: f, Users: f, Resources: f, Credentials: f, Snapshots: f, Sessions: f, Recordings: &f.ingest, Health: f, Pools: f,
		Country: func(netip.Addr) string { return "NL" }}
}
