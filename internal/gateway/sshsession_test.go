package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/werbot/werbot/internal/connector"
)

func init() {
	// a target that runs until the session ends and then, like the SSH
	// connector after a cancel, returns no error and no exit status
	connector.Register(connector.Type{Name: "quiet", NewConfig: func() proto.Message { return &emptypb.Empty{} },
		Validate: func(proto.Message) error { return nil }, Actions: []string{"quiet.connect"},
		Kinds: []connector.Kind{connector.KindExec}, Endpoint: func(proto.Message) connector.Endpoint { return connector.Endpoint{NoHealth: true} }})
	RegisterConnector(funcConn{typ: "quiet", kinds: []Kind{KindExec}, serve: func(ctx context.Context, _ *Session) error {
		<-ctx.Done()
		return nil
	}})
}

// Input class "exec ended by the gateway (revoke, lock, cancel)": the client
// gets no exit status, so a command cut short never reads as exit 0.
func TestExecEndedByRevokeHasNoExitStatus(t *testing.T) {
	e := newSSHEnv(t, Config{})
	e.m["rquiet"] = uuid.New()
	p, err := CompilePolicy(testSnapshot(t, e.m, sshPolicies, nil, res{"rquiet", "quiet", "quiet"}))
	require.NoError(t, err)
	e.f.setPolicy(p)

	s, err := e.client(t, "alice+quiet").NewSession()
	require.NoError(t, err)
	require.NoError(t, s.Start("sleep 30"))
	var id uuid.UUID
	require.Eventually(t, func() bool {
		ids := heartbeatSessionIDs(e.p.active, e.p.spool)
		if len(ids) == 1 {
			id = uuid.MustParse(ids[0])
		}
		return id != uuid.Nil && bound(e.p.active, id)
	}, 2*time.Second, 5*time.Millisecond)

	e.p.active.cancel(id, causeRevoked)
	var missing *gossh.ExitMissingError
	require.ErrorAs(t, s.Wait(), &missing)
	require.Eventually(t, func() bool { return len(e.f.endsOf(id)) == 1 }, 2*time.Second, 5*time.Millisecond)
	require.Equal(t, []string{"access_revoked"}, e.f.endsOf(id))
}
