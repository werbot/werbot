package tunnel

import (
	"context"
	"io"
	"log/slog"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// Codes fakeMFA.VerifyTOTP answers specially; any other code is accepted.
const (
	wrongTOTP     = "000000"
	exhaustedTOTP = "999999"
)

// fakeMFA implements MFA over challenges kept per instance and records the calls.
type fakeMFA struct {
	mu        sync.Mutex
	creates   []MFAChallengeRequest
	verifies  [][3]uuid.UUID          // org, instance, id
	statuses  map[[2]uuid.UUID]string // (instance, id) -> status; missing: ErrNotFound
	confirmed []uuid.UUID             // Confirmed answer
	ages      []string                // the binding of every Age
	age       int64                   // Age answer with method
	method    string
}

func newFakeMFA() *fakeMFA {
	return &fakeMFA{statuses: map[[2]uuid.UUID]string{}, age: math.MaxInt64}
}

func (m *fakeMFA) Create(_ context.Context, r MFAChallengeRequest) (uuid.UUID, string, string, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.creates = append(m.creates, r)
	id := uuid.New()
	m.statuses[[2]uuid.UUID{r.InstanceID, id}] = "pending"
	return id, "BCDF-GHJK", "https://werbot.test/mfa/" + id.String(), time.Unix(1767225600, 0), nil
}

func (m *fakeMFA) VerifyTOTP(_ context.Context, orgID, instanceID, id uuid.UUID, code string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifies = append(m.verifies, [3]uuid.UUID{orgID, instanceID, id})
	if _, ok := m.statuses[[2]uuid.UUID{instanceID, id}]; !ok {
		return false, apperr.ErrNotFound
	}
	switch code {
	case wrongTOTP:
		return false, nil
	case exhaustedTOTP:
		return false, apperr.ErrResourceExhausted
	}
	m.statuses[[2]uuid.UUID{instanceID, id}] = "confirmed"
	return true, nil
}

func (m *fakeMFA) Status(_ context.Context, _, instanceID, id uuid.UUID) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.statuses[[2]uuid.UUID{instanceID, id}]
	if !ok {
		return "", apperr.ErrNotFound
	}
	return s, nil
}

func (m *fakeMFA) Confirmed(context.Context, uuid.UUID, uuid.UUID) ([]uuid.UUID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.confirmed, nil
}

func (m *fakeMFA) Age(_ context.Context, _, _ uuid.UUID, binding string) (int64, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ages = append(m.ages, binding)
	return m.age, m.method, nil
}

// with runs fn under the fake's lock to read or change its state.
func (m *fakeMFA) with(fn func(m *fakeMFA)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fn(m)
}

// newMFABench is a bench whose users and sessions are a realtimeFake and
// whose MFA is a fakeMFA; logs (nil: discarded) gets the handler's log.
func newMFABench(t *testing.T, logs io.Writer) (*bench, *realtimeFake, *fakeMFA) {
	t.Helper()
	fm := newFakeMFA()
	rt := &realtimeFake{bridges: make(chan bridgeCall, 4), tickets: map[uuid.UUID]WebTicket{}}
	b := newBenchTLS(t, nil, nil, func(d *Deps) {
		rt.fake = d.Users.(*fake)
		d.Users, d.Sessions, d.MFA = rt, rt, fm
		if logs != nil {
			d.Logger = slog.New(slog.NewTextHandler(logs, nil))
		}
	})
	return b, rt, fm
}
