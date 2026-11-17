package recorder

import (
	"encoding/json"
	"path"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// sessionEnd is an EndSession waiting in the spool.
type sessionEnd struct {
	reason   string
	counters *gatewayv1.SessionCounters // nil: an end.json written before counters existed
}

// endJSON is the content of end.json; counters are protojson.
type endJSON struct {
	Reason   string          `json:"reason"`
	Counters json.RawMessage `json:"counters,omitempty"`
}

// End spools EndSession(sessionID, reason, c); the id is in PendingEnds until it is
// delivered. c are the final counters of the session (nil: none).
func (s *Spool) End(sessionID uuid.UUID, reason string, c *gatewayv1.SessionCounters) error {
	e := endJSON{Reason: reason}
	if c != nil {
		b, err := protojson.Marshal(c)
		if err != nil {
			return err
		}
		e.Counters = b
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shut {
		return errShutdown
	}
	if s.sessions[sessionID] == nil {
		if err := s.mkdir(sessionID); err != nil {
			return err
		}
		s.sessions[sessionID] = &session{}
	}
	if err := s.write(path.Join(sessionID.String(), endFile), b); err != nil {
		return err
	}
	s.ends[sessionID] = sessionEnd{reason: reason, counters: c}
	s.notify()
	select {
	case s.ended <- struct{}{}:
	default:
	}
	return nil
}

// readEnd reads an end.json left by an earlier process.
func (s *Spool) readEnd(name string) (sessionEnd, error) {
	var e endJSON
	if err := s.readJSON(name, &e); err != nil {
		return sessionEnd{}, err
	}
	out := sessionEnd{reason: e.Reason}
	if len(e.Counters) > 0 {
		out.counters = &gatewayv1.SessionCounters{}
		if err := protojson.Unmarshal(e.Counters, out.counters); err != nil {
			return sessionEnd{}, err
		}
	}
	return out, nil
}

// PendingEnds lists the sessions whose EndSession is not delivered yet, in ascending order.
func (s *Spool) PendingEnds() []uuid.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedKeys(s.ends)
}
