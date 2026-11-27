// Package dbquery runs the queries of the db_query tool: one warm protocol
// session per resource over a gateway stream, opened by the opener of the
// resource type. The gateway logs in to the database and checks every
// query; this package only speaks the protocol.
package dbquery

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// Statuses of a Result.
const (
	StatusOK      = "ok"
	StatusDenied  = "denied"
	StatusPending = "pending_approval"
)

// PendingAfter is how long a query waits for an approval before it answers
// pending_approval.
const PendingAfter = 50 * time.Second

// pendingAfter is PendingAfter; tests shrink it.
var pendingAfter = PendingAfter

// Query is one db_query call: Text runs on Resource; MaxRows above 0 keeps
// only that many rows; ApprovalID is for the transcript only.
type Query struct {
	Resource, Text string
	MaxRows        int
	ApprovalID     string
}

// Result is the outcome of a query: the last result set with columns (rows
// hold strings or nil), else the command tag in Message.
type Result struct {
	Status      string   `json:"status" jsonschema:"ok, denied or pending_approval"`
	Columns     []string `json:"columns"`
	Rows        [][]any  `json:"rows" jsonschema:"each value is text or null"`
	RowCount    int      `json:"row_count"`
	Truncated   bool     `json:"truncated"`
	ApprovalID  string   `json:"approval_id,omitempty"`
	ApprovalURL string   `json:"approval_url,omitempty"`
	Message     string   `json:"message,omitempty"`
}

// Conn is a warm protocol session of one resource; one query at a time.
type Conn interface {
	Query(ctx context.Context, q Query) (Result, error)
	Close() error
}

// Opener does the protocol handshake over a gateway stream; no credential
// is needed: the gateway logs in to the database.
type Opener func(ctx context.Context, conn net.Conn) (Conn, error)

// Dialer opens a gateway stream to resource.
type Dialer func(ctx context.Context, resource string) (net.Conn, error)

// openers are written only in init (and by tests before they run queries), so
// they need no lock.
var openers = map[string]Opener{}

// Register makes o the opener of resource type typ; a second opener of one
// type panics.
func Register(typ string, o Opener) {
	if _, dup := openers[typ]; dup {
		panic("dbquery: opener of " + typ + " registered twice")
	}
	openers[typ] = o
}

var (
	errNoQueries = errors.New("resource does not accept queries")
	errClosed    = errors.New("the agent run ended")
	// errUnsent marks a query that never reached the gateway: the stream
	// ended while the Conn was idle. The Pool runs it again on a new Conn.
	errUnsent = errors.New("query not sent")
)

// sqlError is an error the database answered: the Conn stays usable.
type sqlError struct{ error }

func (e sqlError) Unwrap() error { return e.error }

// Pool keeps one warm Conn per resource for one agent run.
type Pool struct {
	dial   Dialer
	mu     sync.Mutex
	conns  map[string]Conn // idle
	closed bool
}

// NewPool is an empty pool that opens streams with dial.
func NewPool(dial Dialer) *Pool {
	return &Pool{dial: dial, conns: map[string]Conn{}}
}

// Query runs q on the warm Conn of q.Resource, opening one with the opener
// of typ if there is none. A Conn in use by another call is not shared: a
// second Conn opens. An error other than the database's, or a query left
// waiting for an approval, closes the Conn.
func (p *Pool) Query(ctx context.Context, typ string, q Query) (Result, error) {
	open := openers[typ]
	if open == nil {
		return Result{}, errNoQueries
	}
	c, warm, err := p.take(ctx, q.Resource, open)
	if err != nil {
		return Result{}, err
	}
	res, err := c.Query(ctx, q)
	if warm && errors.Is(err, errUnsent) {
		_ = c.Close()
		if c, err = p.open(ctx, q.Resource, open); err != nil {
			return Result{}, err
		}
		res, err = c.Query(ctx, q)
	}
	if _, ok := errors.AsType[sqlError](err); (err != nil && !ok) || res.Status == StatusPending {
		_ = c.Close()
		return res, err
	}
	p.put(q.Resource, c)
	return res, err
}

// take removes the idle Conn of resource from the pool, or opens a new one;
// warm reports an idle one.
func (p *Pool) take(ctx context.Context, resource string, open Opener) (c Conn, warm bool, err error) {
	p.mu.Lock()
	c, warm = p.conns[resource]
	delete(p.conns, resource)
	closed := p.closed
	p.mu.Unlock()
	switch {
	case closed:
		return nil, false, errClosed
	case warm:
		return c, true, nil
	}
	c, err = p.open(ctx, resource, open)
	return c, false, err
}

func (p *Pool) open(ctx context.Context, resource string, open Opener) (Conn, error) {
	nc, err := p.dial(ctx, resource)
	if err != nil {
		return nil, err
	}
	c, err := open(ctx, nc)
	if err != nil {
		_ = nc.Close()
		return nil, err
	}
	return c, nil
}

// put returns c to the pool; one already idle for resource, or a closed
// pool, closes c instead.
func (p *Pool) put(resource string, c Conn) {
	p.mu.Lock()
	_, busy := p.conns[resource]
	keep := !busy && !p.closed
	if keep {
		p.conns[resource] = c
	}
	p.mu.Unlock()
	if !keep {
		_ = c.Close()
	}
}

// Close closes every idle Conn; one in use closes when its query returns.
// The pool runs nothing after Close.
func (p *Pool) Close() error {
	p.mu.Lock()
	conns := p.conns
	p.conns, p.closed = map[string]Conn{}, true
	p.mu.Unlock()
	var errs []error
	for _, c := range conns {
		errs = append(errs, c.Close())
	}
	return errors.Join(errs...)
}
