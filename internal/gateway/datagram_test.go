package gateway

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	_ "github.com/werbot/werbot/internal/connector/tcp" // the server type of "pgdb"
	"github.com/werbot/werbot/internal/platform/sshdgram"
)

// echoDatagrams sends every datagram back to its flow until the client closes
// the channel or ctx ends.
func echoDatagrams(ctx context.Context, d Datagrams) error {
	stop := context.AfterFunc(ctx, func() { _ = d.Close() })
	defer stop()
	buf := make([]byte, sshdgram.MaxPayload)
	for {
		flow, n, err := d.Recv(buf)
		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return err
		}
		if err := d.Send(flow, buf[:n]); err != nil {
			return err
		}
	}
}

// withPortResources replaces the snapshot of e with "denied", "tcp" (streamonly),
// "pg" (plainstream on 10.0.0.5, ports 5432-5433) and "pgdb" (tcp 10.0.0.5:5432).
func (e *sshEnv) withPortResources(t *testing.T) {
	t.Helper()
	e.m["rpg"], e.m["rpgdb"] = uuid.New(), uuid.New()
	s := testSnapshot(t, e.m, sshPolicies, nil, res{"rdenied", "denied", "fake"}, res{"rtcp", "tcp", "streamonly"},
		res{"rpg", "pg", "plainstream"}, res{"rpgdb", "pgdb", "tcp"})
	s.Resources[2].ConfigJson = []byte(`"10.0.0.5"`)
	s.Resources[3].ConfigJson = []byte(`{"host":"10.0.0.5","ports":[{"port":5432}]}`)
	p, err := CompilePolicy(s)
	require.NoError(t, err)
	e.f.setPolicy(p)
}

func openDatagram(c *gossh.Client, host string, port uint32) (gossh.Channel, error) {
	ch, reqs, err := c.OpenChannel(sshdgram.ChannelType, gossh.Marshal(sshdgram.OpenData{Host: host, Port: port, OriginAddr: "127.0.0.1", OriginPort: 40000}))
	if err != nil {
		return nil, err
	}
	go gossh.DiscardRequests(reqs)
	return ch, nil
}

// One datagram channel is one session carrying several flows; each answer goes
// back to its own flow.
func TestDatagramChannelEcho(t *testing.T) {
	e := newSSHEnv(t, Config{})
	e.withPortResources(t)
	ch, err := openDatagram(e.client(t, "alice+pg"), "10.0.0.5", 5432)
	require.NoError(t, err)
	buf := make([]byte, 64)
	for _, d := range []struct {
		flow uint32
		p    string
	}{{1, "query-1"}, {2, "query-2"}, {1, "again"}} {
		require.NoError(t, sshdgram.WriteFrame(ch, d.flow, []byte(d.p)))
		flow, n, err := sshdgram.ReadFrame(ch, buf)
		require.NoError(t, err)
		require.Equal(t, d.flow, flow)
		require.Equal(t, d.p, string(buf[:n]))
	}
	require.Equal(t, 1, e.startCalls(), "one session for all flows")

	require.NoError(t, ch.Close())
	var ends []endCall
	require.Eventually(t, func() bool {
		e.f.with(func(f *fakeControl) { ends = append([]endCall(nil), f.ends...) })
		return len(ends) == 1
	}, 2*time.Second, 5*time.Millisecond)
	require.Equal(t, "closed", ends[0].reason)
	require.EqualValues(t, 3, ends[0].counters.GetDatagramsUp())
	require.EqualValues(t, 3, ends[0].counters.GetDatagramsDown())
}

// A refused datagram channel is Prohibited with the user text and starts no session.
func TestDatagramChannelDenied(t *testing.T) {
	e := newSSHEnv(t, Config{})
	e.withPortResources(t)
	for _, tc := range []struct {
		user, host string
		port       uint32
		text       string
	}{
		{"alice+denied", "10.0.0.5", 53, "Access denied"},
		{"alice+missing", "10.0.0.5", 53, "Access denied"},
		{"alice+pg", "10.0.0.5", 0, "Access denied"},
		{"alice+pg", "10.0.0.6", 5432, "Access denied"},
		{"alice+tcp", "10.0.0.5", 53, "Resource tcp does not support datagram sessions"},
		{"alice", "10.0.0.5", 5432, usageText},
	} {
		_, err := openDatagram(e.client(t, tc.user), tc.host, tc.port)
		var oe *gossh.OpenChannelError
		require.ErrorAs(t, err, &oe, tc.user)
		require.Equal(t, gossh.Prohibited, oe.Reason, tc.user)
		require.Equal(t, tc.text, oe.Message, tc.user)
	}
	require.Zero(t, e.startCalls())
	got := e.p.denials.take()
	require.Len(t, got, 5, "every refused Open is reported; the menu login opens nothing")
	requireValid(t, &gatewayv1.Heartbeat{Denials: got})
}

func TestDatagramChannelMalformedOpenData(t *testing.T) {
	e := newSSHEnv(t, Config{})
	e.withPortResources(t)
	_, _, err := e.client(t, "alice+pg").OpenChannel(sshdgram.ChannelType, []byte{0, 0, 0, 9, 'x'})
	var oe *gossh.OpenChannelError
	require.ErrorAs(t, err, &oe)
	require.Equal(t, gossh.ConnectionFailed, oe.Reason)
	require.Zero(t, e.startCalls())
}

func TestResourceRequest(t *testing.T) {
	e := newSSHEnv(t, Config{})
	e.withPortResources(t)
	ok, payload, err := e.client(t, "alice+pgdb").SendRequest(sshdgram.ResourceRequest, true, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.JSONEq(t, `{"type":"tcp","kinds":["stream"],"ports":[{"port":5432,"end_port":0}]}`, string(payload))

	// forbidden, missing and menu logins read the same: false, no payload
	for _, user := range []string{"alice+denied", "alice+missing", "alice"} {
		ok, payload, err := e.client(t, user).SendRequest(sshdgram.ResourceRequest, true, nil)
		require.NoError(t, err, user)
		require.False(t, ok, user)
		require.Empty(t, payload, user)
	}
	got := e.p.denials.take()
	require.Len(t, got, 2, "forbidden and missing are reported; the menu login is not a refusal")
	for _, d := range got {
		require.Equal(t, "denied", d.GetReason())
		require.Equal(t, e.m["u1"].String(), d.GetUserId())
	}
	require.Equal(t, []string{"denied", "missing"}, []string{got[0].GetResource(), got[1].GetResource()})
	requireValid(t, &gatewayv1.Heartbeat{Denials: got})
	require.Zero(t, e.startCalls(), "a resource request starts no session")
}

// A gateway without a snapshot answers false and reports nothing.
func TestResourceRequestNotReady(t *testing.T) {
	e := newSSHEnv(t, Config{})
	e.withPortResources(t)
	c := e.client(t, "alice+pgdb")
	e.f.with(func(f *fakeControl) { f.connected = false })
	ok, payload, err := c.SendRequest(sshdgram.ResourceRequest, true, nil)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, payload)
	require.Empty(t, e.p.denials.take())
}
