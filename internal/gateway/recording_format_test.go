package gateway

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/werbot/werbot/internal/connector"
)

func init() {
	// querylogger records its streams as querylog: one Event per operation, five of them
	connector.Register(connector.Type{Name: "querylogger", NewConfig: func() proto.Message { return &emptypb.Empty{} },
		Validate: func(proto.Message) error { return nil }, Actions: []string{"querylogger.connect"},
		Kinds: []connector.Kind{connector.KindStream}, Recorded: []connector.Kind{connector.KindStream},
		RecordFormat: connector.RecordQuerylog,
		Endpoint:     func(proto.Message) connector.Endpoint { return connector.Endpoint{AnyTarget: true, NoHealth: true} }})
	RegisterConnector(funcConn{typ: "querylogger", kinds: []Kind{KindStream}, serve: func(_ context.Context, s *Session) error {
		for i := range 5 {
			_ = s.Recording.Event(map[string]int{"n": i})
		}
		return nil
	}})
}

// newQuerylogEnv is a pipeline env that also knows "rq", a querylogger resource;
// rec is the recording factory.
func newQuerylogEnv(t *testing.T, rec RecordingFactory) *pipeEnv {
	t.Helper()
	e := newPipeEnv(t, 10)
	e.m["rq"] = uuid.New()
	e.f.setPolicy(compileTest(t, e.m, pipePolicies, nil, res{"rfake", "fake", "fake"}, res{"rq", "querylog", "querylogger"}))
	e.p.cfg.Recording = rec
	return e
}

// openStream opens a stream of resource name to db:5432.
func openStream(t *testing.T, e *pipeEnv, name string) *Session {
	t.Helper()
	r := e.req(name, KindStream)
	r.Target = "db:5432"
	s, err := e.p.Open(t.Context(), r)
	require.NoError(t, err)
	return s
}

// failingEvents records nothing and refuses Event number fail (1-based).
type failingEvents struct {
	nopSink
	fail  int64
	calls atomic.Int64
}

func (f *failingEvents) Event(any) error {
	if f.calls.Add(1) == f.fail {
		return errors.New("recording lost")
	}
	return nil
}

func TestPipelineRecordingFormat(t *testing.T) {
	formats := make(chan string, 2)
	e := newQuerylogEnv(t, func(_ uuid.UUID, _, _ int, format string) RecordingSink {
		formats <- format
		return nopSink{}
	})
	e.p.abort(openStream(t, e, "querylog"))
	require.Equal(t, connector.RecordQuerylog, <-formats)
	e.p.abort(openStream(t, e, "fake"))
	require.Equal(t, connector.RecordAsciicast, <-formats)
}

// The session end carries the Event lines the recording accepted.
func TestPipelineQueriesInEndSession(t *testing.T) {
	e := newQuerylogEnv(t, func(uuid.UUID, int, int, string) RecordingSink { return &failingEvents{fail: 3} })
	s := openStream(t, e, "querylog")
	c, _ := LookupConnector("querylogger")
	require.NoError(t, e.p.Run(t.Context(), s, c))
	require.Eventually(t, func() bool { return len(e.f.endsOf(s.ID)) == 1 }, 2*time.Second, 5*time.Millisecond)
	r := endResult(s.ID)
	require.NotNil(t, r)
	require.EqualValues(t, 4, r.Queries)

	a := openStream(t, e, "fake")
	e.p.abort(a)
	require.Eventually(t, func() bool { return len(e.f.endsOf(a.ID)) == 1 }, 2*time.Second, 5*time.Millisecond)
	require.Nil(t, endResult(a.ID), "an asciicast stream has no result")
}
