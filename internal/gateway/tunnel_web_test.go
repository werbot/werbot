package gateway

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
)

// pushMsg sends m on the fake's open Connect stream within two seconds.
func pushMsg(t *testing.T, f *fakeTunnel, m *gatewayv1.ConnectResponse) {
	t.Helper()
	select {
	case f.push <- m:
	case <-time.After(2 * time.Second):
		t.Fatal("no open Connect stream")
	}
}

// nextID is the next id a hook received, within two seconds.
func nextID(t *testing.T, ch <-chan uuid.UUID) uuid.UUID {
	t.Helper()
	select {
	case id := <-ch:
		return id
	case <-time.After(2 * time.Second):
		t.Fatal("the hook was not called")
		return uuid.Nil
	}
}

func TestTunnelDispatchesCommands(t *testing.T) {
	e := newTunnelEnv(t, nil, time.Hour)
	opened, ended := make(chan uuid.UUID, 4), make(chan uuid.UUID, 4)
	e.c.onOpenStream = func(id uuid.UUID) { opened <- id }
	e.c.onTerminate = func(id uuid.UUID) { ended <- id }
	e.run(t, nil)
	open := func(id string) *gatewayv1.ConnectResponse {
		return &gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_OpenStream{OpenStream: &gatewayv1.OpenStream{TicketId: id}}}
	}
	terminate := func(id string) *gatewayv1.ConnectResponse {
		return &gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_Terminate{Terminate: &gatewayv1.Terminate{SessionId: id, Reason: "terminated"}}}
	}
	ticket, session := uuid.New(), uuid.New()
	pushMsg(t, e.f, open(ticket.String()))
	pushMsg(t, e.f, terminate(session.String()))
	pushMsg(t, e.f, open("not-a-uuid"))
	pushMsg(t, e.f, terminate("not-a-uuid"))
	pushMsg(t, e.f, snapMsg(testSnap("v2")))

	require.Equal(t, ticket, nextID(t, opened))
	require.Equal(t, session, nextID(t, ended))
	require.Eventually(t, func() bool { return e.c.Policy().Version() == "v2" }, 2*time.Second, 5*time.Millisecond,
		"the stream lives after a bad id")
	require.Empty(t, opened, "a bad id runs no hook")
	require.Empty(t, ended)
	logs := e.log.String()
	require.Contains(t, logs, "ignoring open_stream command")
	require.Contains(t, logs, "ignoring terminate command")
	require.NotContains(t, logs, "not-a-uuid", "ids never reach the log")
	require.NotContains(t, logs, ticket.String())
}

// webTunnel is a TunnelService over HTTP/2 that keeps the calls of the web front.
type webTunnel struct {
	gatewayv1connect.UnimplementedTunnelServiceHandler
	mu     sync.Mutex
	err    error // AuthenticateUser fails with it when set
	answer *gatewayv1.AuthenticateUserResponse
	auths  []*gatewayv1.AuthenticateUserRequest
	starts []*gatewayv1.StartSessionRequest
	send   []*gatewayv1.StreamFrame    // Stream sends these first
	hold   chan struct{}               // when set, Stream reads only after it closes
	frames chan *gatewayv1.StreamFrame // Stream got these; closed at the end of the request
}

func (f *webTunnel) AuthenticateUser(_ context.Context, r *connect.Request[gatewayv1.AuthenticateUserRequest]) (*connect.Response[gatewayv1.AuthenticateUserResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.auths = append(f.auths, r.Msg)
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.answer), nil
}

func (f *webTunnel) StartSession(_ context.Context, r *connect.Request[gatewayv1.StartSessionRequest]) (*connect.Response[gatewayv1.StartSessionResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts = append(f.starts, r.Msg)
	return connect.NewResponse(&gatewayv1.StartSessionResponse{SessionId: testSessionID.String(), Credential: &gatewayv1.Credential{Kind: "password"}}), nil
}

func (f *webTunnel) Stream(ctx context.Context, st *connect.BidiStream[gatewayv1.StreamRequest, gatewayv1.StreamResponse]) error {
	defer close(f.frames)
	if f.hold != nil {
		<-f.hold
		if ctx.Err() != nil { // as the server, whose work (spending the ticket) needs the call alive
			return ctx.Err()
		}
	}
	for _, fr := range f.send {
		if err := st.Send(&gatewayv1.StreamResponse{Frame: fr}); err != nil {
			return err
		}
	}
	for {
		m, err := st.Receive()
		if err != nil {
			return nil
		}
		f.frames <- m.GetFrame()
	}
}

// newWebTunnel serves f over HTTP/2 until the test ends.
func newWebTunnel(t *testing.T, f *webTunnel) *tunnelClient {
	t.Helper()
	f.frames = make(chan *gatewayv1.StreamFrame, 16)
	mux := http.NewServeMux()
	mux.Handle(gatewayv1connect.NewTunnelServiceHandler(f))
	srv := httptest.NewUnstartedServer(mux)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	c := &tunnelClient{callTimeout: time.Second}
	c.conn.Store(&tunnelConn{rpc: gatewayv1connect.NewTunnelServiceClient(srv.Client(), srv.URL)})
	return c
}

func TestStartSessionSendsFront(t *testing.T) {
	f := &webTunnel{}
	c := newWebTunnel(t, f)
	_, _, err := c.StartSession(t.Context(), testUserID, uuid.New(), netip.MustParseAddr("192.0.2.7"), KindTerminal, "", "", nil, nil, startArgs{Front: "web"})
	require.NoError(t, err)
	assert.Equal(t, "web", f.starts[0].GetFront())

	e := newPipeEnv(t, 10)
	s := e.open(t, "fake", KindTerminal)
	assert.Equal(t, startArgs{Front: "ssh", Transport: "tcp"}, sessionAgent(s.ID), "the SSH front starts ssh sessions")
}

func TestAuthenticateWebTicket(t *testing.T) {
	session := uuid.New()
	f := &webTunnel{answer: &gatewayv1.AuthenticateUserResponse{UserId: testUserID.String(), Login: "alice", Country: "NL",
		TeamIds: []string{testTeamID.String()}, Web: &gatewayv1.WebTicket{Mode: "observe", SessionId: session.String(), ClientIp: "203.0.113.7"}}}
	c := newWebTunnel(t, f)
	ticket := uuid.New()
	u, wt, err := c.AuthenticateWebTicket(t.Context(), ticket)
	require.NoError(t, err)
	assert.Equal(t, User{ID: testUserID, Login: "alice", Country: "NL", TeamIDs: []uuid.UUID{testTeamID}}, u)
	assert.Equal(t, WebTicket{Mode: "observe", SessionID: session, ClientIP: netip.MustParseAddr("203.0.113.7")}, wt)
	assert.Equal(t, ticket.String(), f.auths[0].GetWebTicket())

	f.answer.Web = &gatewayv1.WebTicket{Mode: "web", Resource: "db", Cols: 120, Rows: 40, ClientIp: "198.51.100.1"}
	_, wt, err = c.AuthenticateWebTicket(t.Context(), ticket)
	require.NoError(t, err)
	assert.Equal(t, WebTicket{Mode: "web", Resource: "db", Window: Window{120, 40}, ClientIP: netip.MustParseAddr("198.51.100.1")}, wt)

	agent := &gatewayv1.AgentInfo{AgentId: uuid.NewString(), RunId: uuid.NewString(), SponsorUserId: testUserID.String()}
	for name, bad := range map[string]struct {
		web   *gatewayv1.WebTicket
		agent *gatewayv1.AgentInfo
	}{
		"no ticket":      {},
		"bad client ip":  {web: &gatewayv1.WebTicket{Mode: "web", ClientIp: "x"}},
		"bad session id": {web: &gatewayv1.WebTicket{Mode: "observe", SessionId: "x", ClientIp: "192.0.2.1"}},
		"an agent":       {web: &gatewayv1.WebTicket{Mode: "web", ClientIp: "192.0.2.1"}, agent: agent},
	} {
		f.answer.Web, f.answer.Agent = bad.web, bad.agent
		_, _, err := c.AuthenticateWebTicket(t.Context(), ticket)
		assert.Error(t, err, name)
	}
}

func TestAuthenticateWebTicketNotReady(t *testing.T) {
	f := &webTunnel{err: connect.NewError(connect.CodeUnavailable, errors.New("down"))}
	c := newWebTunnel(t, f)
	_, _, err := c.AuthenticateWebTicket(t.Context(), uuid.New())
	require.ErrorIs(t, err, ErrNotReady)

	f.err = connect.NewError(connect.CodeNotFound, errors.New("no ticket"))
	_, _, err = c.AuthenticateWebTicket(t.Context(), uuid.New())
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrNotReady)
}

// nextFrame is the next frame the fake's Stream got, within two seconds; nil once it ended.
func nextFrame(t *testing.T, f *webTunnel) *gatewayv1.StreamFrame {
	t.Helper()
	select {
	case fr := <-f.frames:
		return fr
	case <-time.After(2 * time.Second):
		t.Fatal("no stream frame")
		return nil
	}
}

func TestOpenStreamFrames(t *testing.T) {
	f := &webTunnel{send: []*gatewayv1.StreamFrame{
		{Frame: &gatewayv1.StreamFrame_Data{Data: []byte("hi")}},
		{Frame: &gatewayv1.StreamFrame_Resize{Resize: &gatewayv1.Resize{}}}, // no window: skipped
		{Frame: &gatewayv1.StreamFrame_Resize{Resize: &gatewayv1.Resize{Cols: 100, Rows: 30}}},
		{Frame: &gatewayv1.StreamFrame_Close{Close: &gatewayv1.StreamClose{Reason: "closed"}}},
	}}
	c := newWebTunnel(t, f)
	ticket := uuid.New()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	s, err := c.OpenStream(ctx, ticket)
	require.NoError(t, err)
	require.Equal(t, ticket.String(), nextFrame(t, f).GetOpen().GetTicketId(), "the first frame opens the ticket")

	out := bytes.Repeat([]byte("x"), 40<<10)
	require.NoError(t, s.Write(out))
	require.Len(t, nextFrame(t, f).GetData(), 32<<10, "a frame carries at most 32 KiB")
	require.Len(t, nextFrame(t, f).GetData(), 8<<10)
	require.NoError(t, s.Resize(80, 24))
	assert.Equal(t, int32(80), nextFrame(t, f).GetResize().GetCols())
	require.NoError(t, s.Close("closed"))
	require.NoError(t, s.Close("error"), "Close is idempotent")
	require.Error(t, s.Write([]byte("late")))
	assert.Equal(t, "closed", nextFrame(t, f).GetClose().GetReason())
	assert.Nil(t, nextFrame(t, f), "the request ends after the close")

	d, _, _, err := s.Read()
	require.NoError(t, err)
	assert.Equal(t, []byte("hi"), d)
	_, cols, rows, err := s.Read()
	require.NoError(t, err)
	assert.Equal(t, [2]int{100, 30}, [2]int{cols, rows})
	_, _, _, err = s.Read()
	var closed streamCloseError
	require.ErrorAs(t, err, &closed, "a close from the other side")
}

// The call outlives the caller's context: what was sent before it ended,
// the close frame included, still reaches a server that reads it late.
func TestOpenStreamOutlivesCaller(t *testing.T) {
	f := &webTunnel{hold: make(chan struct{})}
	c := newWebTunnel(t, f)
	ctx, cancel := context.WithCancel(t.Context())
	s, err := c.OpenStream(ctx, uuid.New())
	require.NoError(t, err)
	require.NoError(t, s.Write([]byte("Access denied\r\n")))
	require.NoError(t, s.Close("denied"))
	cancel()
	time.Sleep(50 * time.Millisecond) // the cancellation reaches the transport first
	close(f.hold)
	assert.NotNil(t, nextFrame(t, f).GetOpen())
	assert.Equal(t, "Access denied\r\n", string(nextFrame(t, f).GetData()))
	assert.Equal(t, "denied", nextFrame(t, f).GetClose().GetReason())
	assert.Nil(t, nextFrame(t, f), "the request ends after the close")
}
