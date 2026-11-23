package relay

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	relayv1 "github.com/werbot/werbot/gen/werbot/relay/v1"
	"github.com/werbot/werbot/gen/werbot/relay/v1/relayv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
)

type callerKey struct{}

type caller struct{ orgID, instanceID uuid.UUID }

// WithCaller names the gateway instance calling Bridge: its organization and
// instance must be the ticket's.
func WithCaller(ctx context.Context, orgID, instanceID uuid.UUID) context.Context {
	return context.WithValue(ctx, callerKey{}, caller{orgID, instanceID})
}

// Bridge joins the gateway stream gw for ticketID to the browser waiting for
// it, here or on the ticket's replica over RelayService.Pipe, and returns
// once the stream is done. It spends the ticket first: a ticket that is not
// the caller's (WithCaller) closes gw not_found and stays spent. With no free
// slot for a remote bridge it returns ErrLimit and leaves gw open.
func (r *Relay) Bridge(ctx context.Context, ticketID uuid.UUID, gw Pipe) error {
	t, err := r.tickets.Consume(ctx, ticketID)
	if err != nil && !errors.Is(err, apperr.ErrNotFound) {
		_ = gw.Close(ReasonError)
		return err
	}
	if c, ok := ctx.Value(callerKey{}).(caller); err != nil || !ok || c != (caller{t.OrgID, t.InstanceID}) {
		r.reject("ticket")
		return gw.Close(ReasonNotFound)
	}
	if t.TargetServerID == r.cfg.InstanceID {
		r.handOff(ctx, ticketID, gw)
		return nil
	}
	if !r.acquire() {
		r.reject("limit")
		return ErrLimit
	}
	defer r.release()
	u, err := r.onlineURL(ctx, t.TargetServerID)
	if err != nil {
		r.reject("unavailable")
		_ = gw.Close(ReasonError)
		return nil
	}
	r.link(ctx, u, ticketID, gw)
	return nil
}

// link pipes gw to the replica at url until the replica's side ends; the
// pump from gw ends when gw's stream does.
func (r *Relay) link(ctx context.Context, url string, ticketID uuid.UUID, gw Pipe) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cs := relayv1connect.NewRelayServiceClient(&http.Client{Transport: r.tr}, url, connect.WithReadMaxBytes(pipeReadMaxBytes)).Pipe(ctx)
	remote := &framePipe{
		recv: func() (*relayv1.PipeFrame, error) { m, err := cs.Receive(); return m.GetFrame(), err },
		send: func(f *relayv1.PipeFrame) error { return cs.Send(&relayv1.PipeRequest{Frame: f}) },
	}
	l := &pair{a: gw, b: remote}
	r.track(l)
	defer r.untrack(l)
	if err := remote.send(&relayv1.PipeFrame{Frame: &relayv1.PipeFrame_TicketId{TicketId: ticketID.String()}}); err != nil {
		_ = l.Close(ReasonError)
		return
	}
	go pump(gw, remote, l)
	pump(remote, gw, l)
}

// pair is two joined streams; Close ends both once with one reason.
type pair struct {
	a, b Pipe
	once sync.Once
}

func (l *pair) Close(reason string) error {
	l.once.Do(func() {
		_ = l.a.Close(reason)
		_ = l.b.Close(reason)
	})
	return nil
}

// pump copies src to dst until src ends, then closes the pair with src's
// close reason (ReasonError without one). A failed dst closes the pair with
// ReasonError at once; src is still read to its end, so the close frame
// reaches it before the stream is torn down.
func pump(src, dst Pipe, l *pair) {
	failed := false
	for {
		d, cols, rows, err := src.Read()
		if err != nil {
			_ = l.Close(reasonOf(err))
			return
		}
		switch {
		case failed:
		case cols > 0:
			err = dst.Resize(cols, rows)
		default:
			err = dst.Write(d)
		}
		if err != nil {
			failed = true
			_ = l.Close(ReasonError)
		}
	}
}

// pipeService serves RelayService on the internal listener: a bridging
// replica pipes a gateway stream to the browser waiting here.
type pipeService struct{ r *Relay }

func (h pipeService) Pipe(ctx context.Context, st *connect.BidiStream[relayv1.PipeRequest, relayv1.PipeResponse]) error {
	first, err := st.Receive()
	if err != nil {
		return err
	}
	id, err := uuid.Parse(first.GetFrame().GetTicketId())
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("first frame must name a ticket"))
	}
	h.r.handOff(ctx, id, &framePipe{
		recv: func() (*relayv1.PipeFrame, error) { m, err := st.Receive(); return m.GetFrame(), err },
		send: func(f *relayv1.PipeFrame) error { return st.Send(&relayv1.PipeResponse{Frame: f}) },
	})
	return nil
}

// transport is the HTTP/2-only, TLS 1.3 client of the other replicas: it
// trusts only the internal CA, checks the host of internal_url and presents
// this replica's certificate.
func (r *Relay) transport() *http.Transport {
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(r.ca.RootPEM())
	var protos http.Protocols
	protos.SetHTTP2(true)
	return &http.Transport{
		Protocols:       &protos,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, GetClientCertificate: r.clientCert},
		HTTP2: &http.HTTP2Config{SendPingTimeout: 5 * time.Second, PingTimeout: 5 * time.Second,
			MaxReceiveBufferPerStream: 64 << 10},
	}
}
