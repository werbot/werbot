package gateway

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/gateway/recorder"
)

func init() {
	// a type with exec and shell actions whose connector also serves the console
	connector.Register(connector.Type{Name: "consoley", NewConfig: func() proto.Message { return &structpb.Struct{} },
		Validate: func(proto.Message) error { return nil }, Actions: []string{"consoley.connect", "consoley.exec", "consoley.shell"},
		Kinds:    []connector.Kind{connector.KindTerminal, connector.KindExec},
		Endpoint: func(proto.Message) connector.Endpoint { return connector.Endpoint{NoHealth: true} }})
	RegisterConnector(consoleConn{funcConn{typ: "consoley", kinds: []Kind{KindTerminal, KindExec}, serve: block}})
}

// consoleConn serves sessions like its funcConn and the console with serveConsole.
type consoleConn struct{ funcConn }

func (consoleConn) ServeConsole(ctx context.Context, s *Session) error { return serveConsole(ctx, s) }

// serveConsole writes "console" and runs until the session ends; on the
// resource "x" it checks "mkdir x" in /home/u instead and returns.
func serveConsole(ctx context.Context, s *Session) error {
	w := io.Discard
	if s.Terminal != nil {
		w = s.Terminal
	}
	_, _ = io.WriteString(w, "console\n")
	if s.Resource.Name == "x" {
		chk, err := s.CheckCommand(ctx, "mkdir x", "/home/u", "/home/u", w)
		_, _ = fmt.Fprintf(w, "checked %v %v\n", chk, err)
		return nil
	}
	<-ctx.Done()
	return ctx.Err()
}

// pathsPolicies bind u1 to everything.
const pathsPolicies = `
@id("binding:b")
permit (principal == Werbot::User::"u1", action, resource);
`

// execActions are the exec actions the path rules of the tests cover.
const execActions = `Werbot::Action::"execy.exec", Werbot::Action::"consoley.exec"`

// pathRule is a snapshot path rule (access any) of the short id name and its forbid as the
// server compiles it (internal/policy/compile_paths.go) on execActions.
func pathRule(m ids, name, effect, message string, globs ...string) (*gatewayv1.PathRule, string) {
	if _, ok := m[name]; !ok {
		m[name] = uuid.New()
	}
	id := m[name].String()
	when := fmt.Sprintf("context.path_hits.contains(%q)", id)
	var notes, unless string
	switch effect {
	case "only":
		message, when = cmp.Or(message, "path outside allowed folders"), fmt.Sprintf("context.path_outside.contains(%q)", id)
	case "approval":
		notes, unless = "@obligation(\"approval\")\n", "\nunless { context.approval_granted }"
	}
	if message != "" {
		notes += fmt.Sprintf("@message(%q)\n", message)
	}
	text := fmt.Sprintf("@id(\"path:%s\")\n%sforbid (principal, action in [%s], resource)\nwhen { %s }%s;\n", id, notes, execActions, when, unless)
	return &gatewayv1.PathRule{Id: id, Paths: globs, Access: "any", Effect: effect}, text
}

// consoleRule is the forbid of a console terminal rule on the shells of
// execy and consoley, as the server compiles it.
const consoleRule = `
@id("terminal:t")
@obligation("console")
forbid (principal, action in [Werbot::Action::"execy.shell", Werbot::Action::"consoley.shell"], resource) when { %s }
unless { context.console };
`

// consoleOn is consoleRule on the resources (short names).
func consoleOn(resources ...string) string {
	refs := make([]string, len(resources))
	for i, r := range resources {
		refs[i] = fmt.Sprintf("Werbot::Resource::%q", r)
	}
	return fmt.Sprintf(consoleRule, "["+strings.Join(refs, ", ")+"].contains(resource)")
}

// pathsPolicy compiles policies of version v with path rules over the
// resources "x" (execy, host 10.0.0.5), "c" and "ask" (consoley).
func pathsPolicy(t *testing.T, m ids, v, policies string, rules ...*gatewayv1.PathRule) *Policy {
	t.Helper()
	for _, n := range []string{"rx", "rc", "rask"} {
		if _, ok := m[n]; !ok {
			m[n] = uuid.New()
		}
	}
	s := testSnapshot(t, m, policies, nil, res{"rx", "x", "execy"}, res{"rc", "c", "consoley"}, res{"rask", "ask", "consoley"})
	s.Version, s.PathRules = v, rules
	s.Resources[0].ConfigJson = []byte(`{"host":"10.0.0.5"}`)
	p, err := CompilePolicy(s)
	require.NoError(t, err)
	return p
}

// pathsPipeEnv is a pipeline env on pathsPolicy with the ids of m.
func pathsPipeEnv(t *testing.T, m ids, policies string, rules ...*gatewayv1.PathRule) *pipeEnv {
	t.Helper()
	e := newPipeEnv(t, 10)
	for k, v := range m {
		e.m[k] = v
	}
	e.f.setPolicy(pathsPolicy(t, e.m, "v1", policies, rules...))
	return e
}

// startPaths are the paths StartSession received, by the session id it returned.
var startPaths sync.Map

// started returns a new session id and notes its paths.
func started(paths []string) uuid.UUID {
	id := uuid.New()
	startPaths.Store(id, paths)
	return id
}

// pathsOf is the paths StartSession received for session id.
func pathsOf(id uuid.UUID) []string {
	p, _ := startPaths.Load(id)
	out, _ := p.([]string)
	return out
}

// endResults are the console results EndSession delivered, by session id.
var endResults sync.Map

func noteResult(id uuid.UUID, r *recorder.SessionResult) {
	if r != nil {
		endResults.Store(id, r)
	}
}

func endResult(id uuid.UUID) *recorder.SessionResult {
	r, _ := endResults.Load(id)
	out, _ := r.(*recorder.SessionResult)
	return out
}
