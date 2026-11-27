package dbproxy

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"sync"

	"github.com/google/uuid"
)

// CancelTarget is a live session that a cancel request may interrupt.
type CancelTarget struct {
	ResourceID uuid.UUID                       // a cancel request acts only within this resource
	Cancel     func(ctx context.Context) error // cancels the running operation at the database
	OnCancel   func()                          // runs first, e.g. to log the cancel line
}

// CancelRegistry maps cancel keys to the live sessions of this gateway. The
// zero value is ready; share one per gateway.
type CancelRegistry struct {
	mu      sync.Mutex
	targets map[uint64]CancelTarget
}

// Register adds t under a new random key, never 0; release removes it.
// PostgreSQL clients see the key as BackendKeyData (ProcessID, SecretKey).
func (r *CancelRegistry) Register(t CancelTarget) (key uint64, release func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.targets == nil {
		r.targets = map[uint64]CancelTarget{}
	}
	for {
		var b [8]byte
		_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error
		key = binary.BigEndian.Uint64(b[:])
		if _, taken := r.targets[key]; key != 0 && !taken {
			break
		}
	}
	r.targets[key] = t
	return key, func() {
		r.mu.Lock()
		delete(r.targets, key)
		r.mu.Unlock()
	}
}

// Cancel interrupts the session of key when it belongs to resourceID: OnCancel,
// then Cancel. It reports whether such a session was found; an unknown key or
// a key of another resource cancels nothing.
func (r *CancelRegistry) Cancel(ctx context.Context, key uint64, resourceID uuid.UUID) bool {
	r.mu.Lock()
	t, ok := r.targets[key]
	r.mu.Unlock()
	if !ok || t.ResourceID != resourceID {
		return false
	}
	t.OnCancel()
	_ = t.Cancel(ctx) // the client gets no answer to a cancel request either way
	return true
}
