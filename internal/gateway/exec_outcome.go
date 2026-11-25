package gateway

import (
	"crypto/sha256"
	"hash"
	"sync"

	"github.com/werbot/werbot/internal/gateway/recorder"
)

// execOutcome is what an exec ended with: its exit status and the SHA-256 of
// its recorded output, in the order of the recording's output events.
type execOutcome struct {
	mu   sync.Mutex
	sum  hash.Hash
	code *int32
}

// trackOutcome makes an exec keep its outcome: the recording output passes
// through the hash, the exit status is kept. Any other session is unchanged.
func (s *Session) trackOutcome() {
	if s.Exec == nil {
		return
	}
	o := &execOutcome{sum: sha256.New()}
	s.outcome, s.Recording = o, outcomeSink{s.Recording, o}
	if set := s.Exec.SetExitStatus; set != nil {
		s.Exec.SetExitStatus = func(code int) {
			o.mu.Lock()
			c := int32(code) //nolint:gosec // SSH exit statuses fit in 32 bits
			o.code = &c
			o.mu.Unlock()
			set(code)
		}
	}
}

// result is the exec result for the session end; nil without an outcome.
func (o *execOutcome) result() *recorder.SessionResult {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return &recorder.SessionResult{ExitCode: o.code, OutputSHA256: o.sum.Sum(nil)}
}

// outcomeSink hashes the output it records; the lock keeps the hash in the
// order the recording receives the output.
type outcomeSink struct {
	RecordingSink
	o *execOutcome
}

func (s outcomeSink) Output(p []byte) {
	s.o.mu.Lock()
	defer s.o.mu.Unlock()
	s.o.sum.Write(p)
	s.RecordingSink.Output(p)
}
