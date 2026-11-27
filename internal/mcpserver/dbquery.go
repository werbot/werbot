package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/connectcli"
	"github.com/werbot/werbot/internal/mcpserver/dbquery"
)

// MaxRows is the largest max_rows of db_query.
const MaxRows = 100000

// DBQueryInput is the input of db_query.
type DBQueryInput struct {
	Resource   string `json:"resource" jsonschema:"the resource name from list_resources"`
	Query      string `json:"query" jsonschema:"one statement in the language of the resource"`
	ApprovalID string `json:"approval_id,omitempty" jsonschema:"the approval of an earlier pending_approval answer, for the transcript only"`
	MaxRows    int    `json:"max_rows,omitempty" jsonschema:"keep at most this many rows, up to 100000; 0: only the agent's limit"`
}

func (in DBQueryInput) validate() error {
	switch {
	case in.Resource == "":
		return errors.New("resource is required")
	case in.Query == "":
		return errors.New("query is required")
	case in.MaxRows < 0 || in.MaxRows > MaxRows:
		return fmt.Errorf("max_rows must be between 0 and %d", MaxRows)
	}
	return nil
}

// dbQuery runs in.Query on the run's warm session of in.Resource. A stream
// the gateway refuses with a denial is the denied status; a database error
// is a tool error.
func (s *server) dbQuery(ctx context.Context, c call, in DBQueryInput) (dbquery.Result, string, error) {
	if err := in.validate(); err != nil {
		return dbquery.Result{}, "", err
	}
	r, err := s.mustResource(ctx, c.run, in.Resource)
	if err != nil {
		return dbquery.Result{}, "", err
	}
	res, err := s.pool.Query(ctx, r.GetType(), dbquery.Query{Resource: r.GetName(), Text: in.Query, MaxRows: in.MaxRows, ApprovalID: in.ApprovalID})
	if text, ok := errors.AsType[refusal](err); ok && gatewayRefusals[string(text)] == dbquery.StatusDenied {
		return dbquery.Result{Status: dbquery.StatusDenied, Message: string(text)}, dbquery.StatusDenied, nil
	}
	if err != nil {
		return dbquery.Result{}, "", err
	}
	return res, res.Status, nil
}

// refusal is the gateway's text for a resource or stream it refused.
type refusal string

func (r refusal) Error() string { return string(r) }

// dbDialer opens the streams of the run's db_query pool: an SSH connection
// to the resource's gateway as <login>+<resource>, then direct-tcpip to the
// first port the gateway lists for it.
func (s *server) dbDialer(run *clientauth.AgentProfile) dbquery.Dialer {
	return func(ctx context.Context, name string) (net.Conn, error) {
		r, err := s.mustResource(ctx, run, name)
		if err != nil {
			return nil, err
		}
		client, err := s.dialSSH(ctx, run, r)
		if err != nil {
			return nil, err
		}
		conn, err := openStream(ctx, client, name)
		if err != nil {
			_ = client.Close()
			return nil, err
		}
		return conn, nil
	}
}

// openStream asks client's gateway for resource and opens a direct-tcpip
// stream to its first port, within ctx.
func openStream(ctx context.Context, client *ssh.Client, resource string) (net.Conn, error) {
	stop := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer stop()
	info, refused, err := connectcli.RequestResource(client)
	switch {
	case err != nil:
		return nil, err
	case refused != "":
		return nil, refusal(refused)
	case len(info.Ports) == 0:
		return nil, fmt.Errorf("resource %s has no ports", resource)
	}
	ch, reqs, err := client.OpenChannel("direct-tcpip", connectcli.OpenData(resource, uint32(info.Ports[0].Port), nil))
	if oe, ok := errors.AsType[*ssh.OpenChannelError](err); ok {
		return nil, refusal(oe.Message)
	}
	if err != nil {
		return nil, err
	}
	go ssh.DiscardRequests(reqs)
	go func() { _, _ = io.Copy(io.Discard, ch.Stderr()) }()
	return pipeStream(client, ch), nil
}

// streamConn is a gateway stream as a net.Conn; Close closes its SSH client.
type streamConn struct {
	net.Conn
	client *ssh.Client
}

func (c streamConn) Close() error {
	err := c.Conn.Close()
	_ = c.client.Close()
	return err
}

// pipeStream relays ch through net.Pipe: the protocol clients cancel through
// deadlines, which an SSH channel lacks. The relay ends when either side
// closes.
func pipeStream(client *ssh.Client, ch ssh.Channel) net.Conn {
	local, remote := net.Pipe()
	go func() { _, _ = io.Copy(remote, ch); _ = remote.Close() }()
	go func() { _, _ = io.Copy(ch, remote); _ = ch.CloseWrite() }()
	return streamConn{Conn: local, client: client}
}
