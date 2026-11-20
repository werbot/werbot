package gateway

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/werbot/werbot/internal/connector"
)

func init() {
	// an exec whose output ends with a prefix of the session credential, then exits 3
	connector.Register(connector.Type{Name: "masky", NewConfig: func() proto.Message { return &emptypb.Empty{} },
		Validate: func(proto.Message) error { return nil }, Actions: []string{"masky.connect"},
		Kinds: []connector.Kind{connector.KindExec}, Endpoint: func(proto.Message) connector.Endpoint { return connector.Endpoint{NoHealth: true} }})
	RegisterConnector(funcConn{typ: "masky", kinds: []Kind{KindExec}, serve: func(_ context.Context, s *Session) error {
		_, _ = io.WriteString(s.Exec.Stdout, "out "+string(s.Secret.Value)+" end "+string(s.Secret.Value[:3]))
		s.Exec.SetExitStatus(3)
		return nil
	}})
}

const maskSecret = "S3cr3t-Value!"

func TestMaskEnabled(t *testing.T) {
	for _, tc := range []struct {
		mode        string
		agent, want bool
	}{
		{"all", false, true}, {"all", true, true},
		{"agents", false, false}, {"agents", true, true},
		{"", false, false}, {"", true, true},
		{"none", false, false}, {"none", true, false},
	} {
		require.Equal(t, tc.want, maskEnabled(tc.mode, tc.agent), "mode %q agent %v", tc.mode, tc.agent)
	}
}

// recBuf is a recording sink that keeps the output it records.
type recBuf struct {
	mu  sync.Mutex
	out bytes.Buffer
}

func (r *recBuf) Output(p []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.out.Write(p)
}
func (*recBuf) Input([]byte)                {}
func (*recBuf) Resize(int, int)             {}
func (*recBuf) Close(context.Context) error { return nil }

// leak writes the session credential to the client and to the recording.
func leak(_ context.Context, s *Session) error {
	line := []byte("pw=" + string(s.Secret.Value) + "\n")
	s.Recording.Output(line)
	w := io.Writer(s.Terminal)
	if s.Exec != nil {
		w = s.Exec.Stdout
	}
	_, err := w.Write(line)
	return err
}

// runLeak opens kind on "fake" under mode, runs leak and returns what the
// client and the recording got.
func runLeak(t *testing.T, mode string, kind Kind) (client, recorded string) {
	t.Helper()
	e := newPipeEnv(t, 10)
	e.f.with(func(f *fakeControl) { f.policy.maskOutput = mode })
	s := e.open(t, "fake", kind)
	s.Secret = Secret{Kind: "password", Value: []byte(maskSecret)}
	rec := &recBuf{}
	s.Recording = rec
	var out bytes.Buffer
	if kind == KindExec {
		s.Exec = &Exec{Stdout: &out, Stderr: io.Discard, SetExitStatus: func(int) {}}
	} else {
		s.Terminal = &Terminal{ReadWriter: struct {
			io.Reader
			io.Writer
		}{&bytes.Buffer{}, &out}}
	}
	require.NoError(t, e.p.Run(t.Context(), s, funcConn{typ: "fake", kinds: []Kind{kind}, serve: leak}))
	return out.String(), rec.out.String()
}

// With mask_output "all" an exec's client output is masked; the recording
// keeps the original.
func TestExecOutputMaskedRecordingOriginal(t *testing.T) {
	client, recorded := runLeak(t, "all", KindExec)
	require.Equal(t, "pw=[masked]\n", client)
	require.Equal(t, "pw="+maskSecret+"\n", recorded)
}

// Stage 11 has no agent subjects: "agents" leaves a human's exec unmasked,
// and a terminal is never masked.
func TestMaskAgentsModeHumanUnmasked(t *testing.T) {
	client, _ := runLeak(t, "agents", KindExec)
	require.Equal(t, "pw="+maskSecret+"\n", client)
	client, _ = runLeak(t, "all", KindTerminal)
	require.Equal(t, "pw="+maskSecret+"\n", client)
}

// The SSH front sends the exit status after Run, so the held tail of a
// masked exec reaches the client first.
func TestMaskFlushBeforeExitStatus(t *testing.T) {
	e := newSSHEnv(t, Config{})
	e.m["rmask"] = uuid.New()
	s := testSnapshot(t, e.m, sshPolicies, nil, res{"rmask", "mask", "masky"})
	s.MaskOutput = "all"
	p, err := CompilePolicy(s)
	require.NoError(t, err)
	e.f.with(func(f *fakeControl) { f.policy = p })
	out, code := run(t, e.client(t, "alice+mask"), false, "x")
	require.Equal(t, "out [masked] end s3c", out)
	require.Equal(t, 3, code)
}
