package gateway

import (
	"bytes"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/connector"
)

func init() {
	// a type with an exec action and a gateway connector; its config {"host": …} is the endpoint host
	connector.Register(connector.Type{Name: "execy", NewConfig: func() proto.Message { return &structpb.Struct{} },
		Validate: func(proto.Message) error { return nil }, Actions: []string{"execy.connect", "execy.exec"},
		Kinds: []connector.Kind{connector.KindTerminal, connector.KindExec},
		Endpoint: func(m proto.Message) connector.Endpoint {
			return connector.Endpoint{Host: m.(*structpb.Struct).GetFields()["host"].GetStringValue(), NoHealth: true}
		}})
	RegisterConnector(funcConn{typ: "execy", kinds: []Kind{KindTerminal, KindExec}, serve: spy})
}

// execTestPolicies bind u1 to everything and refuse destructive commands with a
// message and egress commands without one.
const execTestPolicies = `
@id("binding:b")
permit (principal == Werbot::User::"u1", action, resource);
@id("command:a")
@message("No rm")
forbid (principal, action == Werbot::Action::"execy.exec", resource) when { context.classes.contains("destructive") };
@id("command:e")
forbid (principal, action == Werbot::Action::"execy.exec", resource) when { context.classes.contains("egress") };`

// execPolicy compiles policies of version v over the resources "x" (execy, host
// 10.0.0.5), "fake" (no exec action) and "ssh" (no gateway connector here).
func execPolicy(t *testing.T, m ids, v, policies string) *Policy {
	t.Helper()
	for _, n := range []string{"rx", "rfake", "rssh"} {
		if _, ok := m[n]; !ok {
			m[n] = uuid.New()
		}
	}
	s := testSnapshot(t, m, policies, nil, res{"rx", "x", "execy"}, res{"rfake", "fake", "fake"}, res{"rssh", "ssh", "ssh"})
	s.Version = v
	s.Resources[0].ConfigJson = []byte(`{"host":"10.0.0.5"}`)
	p, err := CompilePolicy(s)
	require.NoError(t, err)
	return p
}

// newExecEnv is a pipeline env on execTestPolicies.
func newExecEnv(t *testing.T) *pipeEnv {
	t.Helper()
	e := newPipeEnv(t, 10)
	e.f.setPolicy(execPolicy(t, e.m, "v1", execTestPolicies))
	return e
}

// cmdReq is an Open request of kind on resource running command.
func (e *pipeEnv) cmdReq(resource string, kind Kind, command string) OpenRequest {
	r := e.req(resource, kind)
	r.Command = command
	return r
}

// startCmd is how fakeControl.starts shows the command of a StartSession; "" without one.
func startCmd(command string, classes []string) string {
	if command == "" && classes == nil {
		return ""
	}
	return fmt.Sprintf(" %q %v", command, classes)
}

func (e *pipeEnv) lastStart() string {
	var s string
	e.f.with(func(f *fakeControl) { s = f.starts[len(f.starts)-1] })
	return s
}

func TestExecStepDeny(t *testing.T) {
	e := newExecEnv(t)
	_, err := e.p.Open(t.Context(), e.cmdReq("x", KindExec, "rm -rf /"))
	cd, ok := errors.AsType[*CommandDeniedError](err)
	require.True(t, ok, "%v", err)
	require.Equal(t, "Command denied: No rm", cd.Error())
	require.ErrorIs(t, err, ErrDenied)
	require.Zero(t, e.startCalls(), "a denied command never reaches StartSession")
	require.Zero(t, e.p.active.used())
	ds := e.p.denials.take()
	require.Len(t, ds, 1)
	require.Equal(t, "denied", ds[0].GetReason())
	require.Equal(t, "exec", ds[0].GetKind())
	require.Equal(t, "rm -rf /", ds[0].GetTarget(), "the denial names the command")
}

func TestExecStepAllowPassesCommand(t *testing.T) {
	e := newExecEnv(t)
	s, err := e.p.Open(t.Context(), e.cmdReq("x", KindExec, "ls -la"))
	require.NoError(t, err)
	require.Equal(t, "ls -la", s.Command)
	require.Equal(t, []string{"read"}, s.Classes)
	require.Equal(t, fmt.Sprintf("%s %s 192.0.2.7", e.m["u1"], e.m["rx"])+startCmd("ls -la", []string{"read"}), e.lastStart())
}

func TestExecStepDenyWithoutMessage(t *testing.T) {
	e := newExecEnv(t)
	_, err := e.p.Open(t.Context(), e.cmdReq("x", KindExec, "curl http://example.com/"))
	require.ErrorIs(t, err, ErrDenied)
	require.Equal(t, "Access denied", openText(err, "x", KindExec, nil), "a rule without a message reads as a plain deny")
}

func TestTerminalWithCommandIsAuthorized(t *testing.T) {
	e := newExecEnv(t)
	_, err := e.p.Open(t.Context(), e.cmdReq("x", KindTerminal, "rm -rf /"))
	require.Equal(t, "Command denied: No rm", openText(err, "x", KindTerminal, nil))
	require.Zero(t, e.startCalls())

	s, err := e.p.Open(t.Context(), e.req("x", KindTerminal)) // a shell: no command to classify
	require.NoError(t, err)
	require.Empty(t, s.Command)
	require.Nil(t, s.Classes)
	require.Equal(t, fmt.Sprintf("%s %s 192.0.2.7", e.m["u1"], e.m["rx"]), e.lastStart())
}

func TestExecStepSkippedWithoutExecAction(t *testing.T) {
	e := newExecEnv(t)
	s, err := e.p.Open(t.Context(), e.cmdReq("fake", KindTerminal, "rm -rf /"))
	require.NoError(t, err, "the type runs no commands: its connector refuses them")
	require.Equal(t, "rm -rf /", s.Command)
	require.Nil(t, s.Classes)
	require.Equal(t, fmt.Sprintf("%s %s 192.0.2.7", e.m["u1"], e.m["rfake"]), e.lastStart(), "no command authorized")
}

func TestExecStepUsesResourceHost(t *testing.T) {
	e := newExecEnv(t)
	s, err := e.p.Open(t.Context(), e.cmdReq("x", KindExec, "curl http://10.0.0.5/"))
	require.NoError(t, err, "the resource host is not egress")
	require.Equal(t, []string{"read"}, s.Classes)
	_, err = e.p.Open(t.Context(), e.cmdReq("x", KindExec, "curl http://10.0.0.6/"))
	require.ErrorIs(t, err, ErrDenied)
}

func TestExecStepAfterConnectorCheck(t *testing.T) {
	e := newExecEnv(t)
	_, err := e.p.Open(t.Context(), e.cmdReq("ssh", KindExec, "rm -rf /"))
	require.ErrorIs(t, err, ErrUnsupportedType, "the type check comes before the command")
	require.NotErrorAs(t, err, new(*CommandDeniedError))
}

func TestCommandDeniedErrorText(t *testing.T) {
	for _, tc := range []struct {
		err  *CommandDeniedError
		want string
		is   error
	}{
		{&CommandDeniedError{Err: ErrDenied, Message: "No rm"}, "Command denied: No rm", ErrDenied},
		{&CommandDeniedError{Err: ErrDenied}, "Access denied", ErrDenied},
		{&CommandDeniedError{Err: ErrApprovalDenied}, "Command denied: approval denied", ErrApprovalDenied},
		{&CommandDeniedError{Err: ErrApprovalDenied, Message: "ask ops"}, "Command denied: ask ops", ErrApprovalDenied},
	} {
		require.Equal(t, tc.want, tc.err.Error())
		require.ErrorIs(t, tc.err, tc.is)
		require.Equal(t, tc.want, openText(tc.err, "x", KindExec, nil))
	}
	require.Equal(t, "Approval timed out", ErrApprovalTimeout.Error())
}

// Every refusal reason passes the heartbeat validation; a command target is cut
// to 1024 bytes, keeping UTF-8 whole.
func TestDenialReasonsPassValidation(t *testing.T) {
	long := strings.Repeat("é", 700)
	for err, want := range map[error]string{
		ErrDenied: "denied",
		&CommandDeniedError{Err: ErrDenied, Message: "No rm"}: "denied",
		&CommandDeniedError{Err: ErrApprovalDenied}:           "approval_denied",
		ErrApprovalTimeout:  "approval_timeout",
		ErrBusy:             "busy",
		ErrSpoolMax:         "spool_full",
		ErrTargetNotAllowed: "target_not_allowed",
		ErrUnsupportedType:  "unsupported_type",
		ErrUnsupportedKind:  "unsupported_kind",
	} {
		d := newDenial(denialReason(err), uuid.New(), "SHA256:x", "db", KindExec, long, netip.MustParseAddr("192.0.2.7"))
		require.Equal(t, want, d.GetReason(), err.Error())
		require.Equal(t, strings.Repeat("é", 512), d.GetTarget())
		requireValid(t, &gatewayv1.Heartbeat{Denials: []*gatewayv1.Denial{d}})
	}
}

// runOut runs cmd as run does and also returns stderr.
func runOut(t *testing.T, c *gossh.Client, pty bool, cmd string) (stdout, stderr string, code int) {
	t.Helper()
	s, err := c.NewSession()
	require.NoError(t, err)
	defer s.Close() //nolint:errcheck // already closed by the server
	if pty {
		require.NoError(t, s.RequestPty("xterm", 40, 120, gossh.TerminalModes{}))
	}
	var out, errOut bytes.Buffer
	s.Stdout, s.Stderr = &out, &errOut
	require.NoError(t, s.Start(cmd))
	code = exitCode(t, s.Wait())
	return out.String(), errOut.String(), code
}

func newSSHExecEnv(t *testing.T) *sshEnv {
	t.Helper()
	e := newSSHEnv(t, Config{})
	p := execPolicy(t, e.m, "v1", execTestPolicies)
	e.f.with(func(f *fakeControl) { f.policy = p })
	return e
}

func TestSSHFrontCommandDeniedText(t *testing.T) {
	e := newSSHExecEnv(t)
	c := e.client(t, "alice+x")
	out, errOut, code := runOut(t, c, false, "rm -rf /")
	require.Empty(t, out)
	require.Equal(t, "Command denied: No rm\r\n", errOut, "an exec reads the refusal on stderr")
	require.Equal(t, 1, code)
	out, _, code = runOut(t, c, true, "rm -rf /")
	require.Equal(t, "Command denied: No rm\r\n", out, "a terminal reads it in the PTY")
	require.Equal(t, 1, code)
	require.Zero(t, e.startCalls())
}

// The connector runs exactly the command the PDP decided, as an exec and in a PTY.
func TestExecRunsAuthorizedCommand(t *testing.T) {
	e := newSSHExecEnv(t)
	c := e.client(t, "alice+x")
	out, code := run(t, c, false, "ls -la")
	require.Equal(t, "spy:x kind=exec cmd=\"ls -la\"\n", out)
	require.Equal(t, 7, code)
	out, _ = run(t, c, true, "ls -la")
	require.Contains(t, out, `spy:x kind=terminal term="xterm" cmd="ls -la"`)
	e.f.with(func(f *fakeControl) {
		want := fmt.Sprintf("%s %s 127.0.0.1", e.m["u1"], e.m["rx"]) + startCmd("ls -la", []string{"read"})
		require.Equal(t, []string{want, want}, f.starts)
	})
}

func (e *pipeEnv) startCalls() int {
	var n int
	e.f.with(func(f *fakeControl) { n = f.startCalls })
	return n
}
