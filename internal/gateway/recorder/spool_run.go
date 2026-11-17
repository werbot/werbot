package recorder

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"path"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

// errUnreadable is a spooled chunk that cannot be read back; the session moves aside.
var errUnreadable = errors.New("spool: chunk file unreadable")

// Run delivers the spool until ctx ends, then returns nil. Every pass first sends
// the pending EndSession calls, so a server whose recording store is down does not
// hold back session ends; then it uploads each session's chunks in order and
// finishes the closed ones. A failed pass is retried after a doubling pause.
//
// ponytail: one sequential uploader (about one 64 KiB chunk per round trip);
// upload sessions in parallel if stage 7a shows the spool growing.
func (s *Spool) Run(ctx context.Context) error {
	var pause time.Duration
	for {
		err := s.pass(ctx)
		if ctx.Err() != nil {
			return nil
		}
		wake, retry := s.wake, (<-chan time.Time)(nil)
		if err != nil {
			pause = min(max(2*pause, s.opts.RetryMin), s.opts.RetryMax)
			wait := min(jitter(pause), s.opts.RetryMax)
			s.log.Warn("spool: upload failed, retrying", "error", err, "retry_in", wait)
			// new chunks do not cut the pause short, a new session end does: a pass
			// delivers the ends first, even while the server cannot store chunks
			wake, retry = nil, time.After(wait)
		} else {
			pause = 0
		}
		select {
		case <-ctx.Done():
			return nil
		case <-wake:
		case <-s.ended:
		case <-retry:
		}
	}
}

// jitter spreads d by ±20 %, so gateways do not retry in step after a server restart.
func jitter(d time.Duration) time.Duration {
	return time.Duration(float64(d) * (0.8 + 0.4*rand.Float64())) //nolint:gosec // jitter, not security
}

// pass makes one delivery round; it stops at the first retryable failure.
func (s *Spool) pass(ctx context.Context) error {
	for _, id := range s.PendingEnds() {
		s.mu.Lock()
		reason, ok := s.ends[id]
		s.mu.Unlock()
		if !ok {
			continue
		}
		if err := s.up.EndSession(ctx, id, reason); err != nil {
			if !refused(err) {
				return err
			}
			s.reject(id, err)
			continue
		}
		s.endDelivered(id)
	}
	s.mu.Lock()
	ids := sortedKeys(s.sessions)
	s.mu.Unlock()
	for _, id := range ids {
		if err := s.upload(ctx, id); err != nil {
			if !refused(err) {
				return err
			}
			s.reject(id, err)
		}
	}
	return nil
}

// upload sends the unsent chunks of a session, then finishes a closed recording
// whose end is delivered and removes its directory.
func (s *Spool) upload(ctx context.Context, id uuid.UUID) error {
	for {
		s.mu.Lock()
		ss := s.sessions[id]
		if ss == nil {
			s.mu.Unlock()
			return nil
		}
		_, ended := s.ends[id]
		st, seq := ss.st, ss.sent
		s.mu.Unlock()
		if seq < st.Chunks {
			if err := s.uploadChunk(ctx, id, seq); err != nil {
				return err
			}
			continue
		}
		if !st.Closed || ended {
			return nil
		}
		if st.Chunks > 0 {
			hash, err := hex.DecodeString(st.FinalHash)
			if err != nil {
				return fmt.Errorf("%w: final hash: %w", errUnreadable, err)
			}
			if err := s.up.FinishRecording(ctx, Manifest{SessionID: id, Chunks: st.Chunks, Bytes: st.Bytes, FinalHash: hash}); err != nil {
				return err
			}
		}
		s.mu.Lock()
		s.removeLocked(id)
		s.mu.Unlock()
		return nil
	}
}

// uploadChunk sends chunk seq; AlreadyExists means an earlier attempt got through.
func (s *Spool) uploadChunk(ctx context.Context, id uuid.UUID, seq int) error {
	name := chunkName(id, seq)
	b, err := s.root.ReadFile(name)
	if err != nil {
		return fmt.Errorf("%w: %w", errUnreadable, err)
	}
	if len(b) <= hashLen {
		return fmt.Errorf("%w: %s is too short", errUnreadable, name)
	}
	if err := s.up.UploadChunk(ctx, id, seq, b[:hashLen], b[hashLen:]); err != nil && connect.CodeOf(err) != connect.CodeAlreadyExists {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.root.Remove(name); err != nil {
		s.log.Warn("spool: remove uploaded chunk", "session_id", id, "seq", seq, "error", err)
	}
	if ss := s.sessions[id]; ss != nil && ss.sent == seq {
		n := int64(len(b) - hashLen)
		ss.sent, ss.pending = seq+1, ss.pending-n
		s.addTotal(-n)
	}
	return nil
}

// endDelivered drops end.json; a directory without a recording goes with it.
func (s *Spool) endDelivered(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ends, id)
	if err := s.root.Remove(path.Join(id.String(), endFile)); err != nil {
		s.log.Warn("spool: remove delivered end", "session_id", id, "error", err)
	}
	if ss := s.sessions[id]; ss == nil || !ss.stored {
		s.removeLocked(id)
	}
}

// removeLocked drops a delivered session; s.mu is held.
func (s *Spool) removeLocked(id uuid.UUID) {
	if err := s.root.RemoveAll(id.String()); err != nil {
		s.log.Warn("spool: remove session", "session_id", id, "error", err)
	}
	if ss := s.sessions[id]; ss != nil {
		s.addTotal(-ss.pending)
	}
	delete(s.sessions, id)
}

// reject moves a session the server refused (or that cannot be read) to
// rejected/ and stops delivering it.
func (s *Spool) reject(id uuid.UUID, cause error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dst := path.Join(rejectedDir, id.String())
	if _, err := s.root.Lstat(dst); err == nil { // refused before: keep that copy too
		dst += "." + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	err := s.root.MkdirAll(rejectedDir, 0o700)
	if err == nil {
		err = s.root.Rename(id.String(), dst)
	}
	s.log.Error("spool: session refused, moved to "+rejectedDir, "session_id", id, "cause", cause, "error", err)
	if ss := s.sessions[id]; ss != nil {
		s.addTotal(-ss.pending)
	}
	delete(s.sessions, id)
	delete(s.ends, id)
}

// refused reports an answer that a retry cannot change.
func refused(err error) bool {
	switch connect.CodeOf(err) {
	case connect.CodeFailedPrecondition, connect.CodeInvalidArgument, connect.CodePermissionDenied, connect.CodeNotFound:
		return true
	}
	return errors.Is(err, errUnreadable)
}
