package recorder

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"

	"github.com/werbot/werbot/internal/platform/metrics"
)

// DefaultSpoolLimit is how many unsent chunk bytes one session may keep in the spool.
const DefaultSpoolLimit = 64 << 20

// ErrSpoolFull is an Append over the session's spool limit.
var ErrSpoolFull = errors.New("recorder: session spool limit exceeded")

// errRejected is an Append to a session whose recording the server refused.
var errRejected = errors.New("recorder: the server refused this session's recording")

// Manifest closes a recording on the server.
type Manifest struct {
	SessionID uuid.UUID
	Chunks    int
	Bytes     int64
	FinalHash []byte
}

// Uploader delivers the spool to the server; the gateway tunnel client implements it.
type Uploader interface {
	UploadChunk(ctx context.Context, sessionID uuid.UUID, seq int, hash, gz []byte) error
	EndSession(ctx context.Context, sessionID uuid.UUID, reason string, c *gatewayv1.SessionCounters) error // a repeat returns nil; nil c keeps the stored counters
	FinishRecording(ctx context.Context, m Manifest) error
}

// SpoolOptions tune a Spool.
type SpoolOptions struct {
	Limit              int64          // unsent bytes per session; 0 → DefaultSpoolLimit
	RetryMin, RetryMax time.Duration  // pause after a failed upload pass (doubling, jittered, at most RetryMax); 0 → 1s and 30s
	Gauge              *metrics.Gauge // set to Total() on every change; nil → none
}

const (
	hashLen     = 32
	rejectedDir = "rejected"
	stateFile   = "state.json"
	endFile     = "end.json"
	chunkExt    = ".chunk"
	tmpExt      = ".tmp"
)

// state is a session's state.json: the totals of every chunk appended so far.
type state struct {
	Chunks    int    `json:"chunks"`
	Bytes     int64  `json:"bytes"`
	FinalHash string `json:"final_hash"` // hex
	// Closed lives in memory only: restore counts every session as closed.
	Closed bool `json:"-"`
}

// session is the in-memory view of one session directory.
type session struct {
	st      state
	stored  bool  // state.json exists
	sent    int   // lowest seq not uploaded yet
	pending int64 // gz bytes of the unsent chunks
}

// Spool keeps recording chunks and session ends on disk until Run delivers them:
// <dir>/<session>/<seq %08d>.chunk (hash + gz), state.json and end.json.
type Spool struct {
	root  *os.Root
	up    Uploader
	opts  SpoolOptions
	log   *slog.Logger
	wake  chan struct{} // any new work
	ended chan struct{} // a new session end: also cuts a retry pause short
	total atomic.Int64
	// writeFailed: the last write or mkdir failed (disk full or not writable);
	// set under mu, read without it on every Pipeline.Open
	writeFailed atomic.Bool

	// ponytail: one lock covers the disk writes of all sessions (an fsync per chunk
	// serialises appends). Stage 7a profiled spool disk work at ~37 % of gateway
	// CPU in a Docker VM, mostly fsync; 7 fsyncs per short recorded session stay
	// (durability). Per-session locks if a mutex profile on reference hardware
	// shows appends waiting on mu.
	mu       sync.Mutex
	sessions map[uuid.UUID]*session
	ends     map[uuid.UUID]sessionEnd // EndSession not delivered yet
	// ponytail: rejected is kept for the process lifetime (16 bytes per refused
	// session, rare); prune it once the spool learns of a session's Recording.Close.
	rejected map[uuid.UUID]struct{} // refused sessions: their later chunks fail
	shut     bool                   // Shutdown ran: the directory may belong to a new Spool
}

// errShutdown is a write to a spool after Shutdown.
var errShutdown = errors.New("recorder: spool is shut down")

// OpenSpool opens (or creates) the spool in dir and restores what an earlier
// process left there; every restored session counts as closed.
func OpenSpool(dir string, up Uploader, opts SpoolOptions, logger *slog.Logger) (*Spool, error) {
	if opts.Limit == 0 {
		opts.Limit = DefaultSpoolLimit
	}
	if opts.RetryMin == 0 {
		opts.RetryMin = time.Second
	}
	if opts.RetryMax == 0 {
		opts.RetryMax = 30 * time.Second
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("spool: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("spool: %w", err)
	}
	s := &Spool{root: root, up: up, opts: opts, log: logger, wake: make(chan struct{}, 1), ended: make(chan struct{}, 1),
		sessions: map[uuid.UUID]*session{}, ends: map[uuid.UUID]sessionEnd{}, rejected: map[uuid.UUID]struct{}{}}
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("spool: %w", err)
	}
	for _, e := range entries {
		id, err := uuid.Parse(e.Name())
		if !e.IsDir() || err != nil || id.String() != e.Name() {
			continue // rejected/ and anything else
		}
		if err := s.restore(id); err != nil {
			s.reject(id, err) // one unreadable session must not stop the gateway
		}
	}
	s.addTotal(0)
	return s, nil
}

// Append stores chunk seq of a session; it does not retain hash or gz.
func (s *Spool) Append(sessionID uuid.UUID, seq int, hash, gz []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shut {
		return errShutdown
	}
	if _, ok := s.rejected[sessionID]; ok {
		return errRejected
	}
	ss := s.sessions[sessionID]
	var pending int64
	if ss != nil {
		pending = ss.pending
	}
	n := int64(len(gz))
	if pending+n > s.opts.Limit {
		return ErrSpoolFull
	}
	if ss == nil {
		if err := s.mkdir(sessionID); err != nil {
			return err
		}
		ss = &session{sent: seq}
		s.sessions[sessionID] = ss
	}
	name := chunkName(sessionID, seq)
	if err := s.write(name, hash, gz); err != nil {
		return err
	}
	st := ss.st
	st.Chunks, st.Bytes, st.FinalHash = seq+1, st.Bytes+n, hex.EncodeToString(hash)
	if err := s.writeState(sessionID, st); err != nil {
		return err // the chunk file stays: restore counts a chunk the state lacks, while deleting it could lose the chain
	}
	ss.st, ss.stored, ss.pending = st, true, ss.pending+n
	s.addTotal(n)
	s.notify()
	return nil
}

// Close marks the recording complete: Run finishes it once its chunks and end are
// delivered. It writes nothing: a restart counts every spooled session as closed.
func (s *Spool) Close(sessionID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shut {
		return errShutdown
	}
	ss := s.sessions[sessionID]
	if ss == nil {
		return nil // nothing was recorded
	}
	ss.st.Closed = true
	s.notify()
	return nil
}

// Shutdown closes the spool directory; later Append, Close and End fail. Run must
// have returned.
func (s *Spool) Shutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shut = true
	return s.root.Close()
}

// Pending is the unsent gz bytes of a session.
func (s *Spool) Pending(sessionID uuid.UUID) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ss := s.sessions[sessionID]; ss != nil {
		return ss.pending
	}
	return 0
}

// Total is the unsent gz bytes of all sessions, spool/rejected excluded.
func (s *Spool) Total() int64 { return s.total.Load() }

func (s *Spool) addTotal(d int64) {
	v := s.total.Add(d)
	if s.opts.Gauge != nil {
		s.opts.Gauge.Set(v)
	}
}

func (s *Spool) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func sortedKeys[V any](m map[uuid.UUID]V) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	return ids
}
