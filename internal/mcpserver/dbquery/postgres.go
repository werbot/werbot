package dbquery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func init() { Register("postgres", openPostgres) }

// pgSettings pin every connection setting the PG* environment could
// otherwise supply: plain protocol 3.0 to the gateway, no TLS, no password,
// no database (the resource's default). Quoted empty values clear a key.
const pgSettings = "host=127.0.0.1 port=5432 user=werbot dbname='' sslmode=disable sslcertmode=allow sslnegotiation=postgres " +
	"target_session_attrs=any min_protocol_version=3.0 max_protocol_version=3.0 channel_binding=prefer " +
	"require_auth='' passfile='' sslrootcert='' sslcert='' sslkey=''"

// closeTimeout bounds the Terminate of Close.
const closeTimeout = 5 * time.Second

// pgDenied is the SQLSTATE of a refused query (the gateway's werbot: query
// denied, or the database's own permission error).
const pgDenied = "42501"

var (
	// approvalNotice is the gateway's NOTICE before it holds a query.
	approvalNotice = regexp.MustCompile(`^Waiting for approval (\S+), timeout \S+: (\S+)$`)
	// truncatedNotice starts the gateway's WARNING when it cut a result.
	truncatedNotice = "werbot: result truncated"
)

const pendingMessage = "waiting for an approval; call db_query again with the same resource and query once it is approved"

// pgConn is a warm PostgreSQL session through the gateway.
type pgConn struct {
	pg   *pgconn.PgConn
	wait *wait // of the running query; nil between queries
}

// wait is the state the notices of one query set.
type wait struct {
	cancel    context.CancelFunc
	truncated bool
	id, url   string
	timer     *time.Timer
	fired     atomic.Bool // the approval wait passed pendingAfter: the query was cancelled
}

func openPostgres(ctx context.Context, conn net.Conn) (Conn, error) {
	cfg, err := pgconn.ParseConfig(pgSettings)
	if err != nil {
		return nil, fmt.Errorf("postgres: connection settings: %w", err)
	}
	c := &pgConn{}
	cfg.Database, cfg.User, cfg.Password, cfg.TLSConfig, cfg.RuntimeParams = "", "werbot", "", nil, map[string]string{}
	cfg.Fallbacks, cfg.ValidateConnect, cfg.AfterConnect, cfg.AfterNetConnect, cfg.OAuthTokenProvider = nil, nil, nil, nil, nil
	cfg.DialFunc = func(context.Context, string, string) (net.Conn, error) { return conn, nil }
	cfg.OnNotice = func(_ *pgconn.PgConn, n *pgconn.Notice) { c.notice(n) }
	if c.pg, err = pgconn.ConnectConfig(ctx, cfg); err != nil {
		return nil, err
	}
	return c, nil
}

// notice reads the gateway's notices of the running query: truncation, and
// the approval hold, which starts the pendingAfter timer.
func (c *pgConn) notice(n *pgconn.Notice) {
	w := c.wait
	if w == nil {
		return
	}
	if strings.HasPrefix(n.Message, truncatedNotice) {
		w.truncated = true
		return
	}
	if m := approvalNotice.FindStringSubmatch(n.Message); m != nil && w.timer == nil {
		w.id, w.url = m[1], m[2]
		w.timer = time.AfterFunc(pendingAfter, func() { w.fired.Store(true); w.cancel() })
	}
}

// Query runs q.Text as one simple-protocol Query. A query held for an
// approval longer than pendingAfter is cancelled, which ends the session.
func (c *pgConn) Query(ctx context.Context, q Query) (Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	w := &wait{cancel: cancel}
	c.wait = w
	defer func() {
		c.wait = nil
		if w.timer != nil {
			w.timer.Stop()
		}
	}()
	res, err := readResults(c.pg.Exec(ctx, q.Text), q.MaxRows)
	res.Truncated = res.Truncated || w.truncated
	return c.outcome(res, err, w)
}

// outcome sorts the end of a query: held past pendingAfter, denied, a
// database error on a usable session, a stream that ended while idle.
func (c *pgConn) outcome(res Result, err error, w *wait) (Result, error) {
	if err == nil {
		return res, nil
	}
	pgErr, isPg := errors.AsType[*pgconn.PgError](err)
	switch {
	case w.fired.Load():
		return Result{Status: StatusPending, ApprovalID: w.id, ApprovalURL: w.url, Message: pendingMessage}, nil
	case isPg && pgErr.Code == pgDenied:
		return Result{Status: StatusDenied, Message: pgErr.Message}, nil
	case isPg && !c.pg.IsClosed():
		return Result{}, sqlError{err}
	case pgconn.SafeToRetry(err):
		return Result{}, fmt.Errorf("%w: %w", errUnsent, err)
	}
	return Result{}, err
}

// readResults reads every result of mrr: the last set with columns, or the
// last command tag; past maxRows (above 0) rows are read and dropped.
func readResults(mrr *pgconn.MultiResultReader, maxRows int) (Result, error) {
	res := Result{Status: StatusOK, Columns: []string{}, Rows: [][]any{}}
	sets := false
	for mrr.NextResult() {
		rr := mrr.ResultReader()
		if fields := rr.FieldDescriptions(); fields != nil {
			res, sets = readSet(rr, fields, maxRows), true
			continue
		}
		tag, err := rr.Close()
		if err == nil && !sets {
			res.RowCount, res.Message = int(tag.RowsAffected()), tag.String()
		}
	}
	return res, mrr.Close()
}

// readSet reads one result set, keeping up to maxRows rows when above 0.
func readSet(rr *pgconn.ResultReader, fields []pgconn.FieldDescription, maxRows int) Result {
	res := Result{Status: StatusOK, Columns: make([]string, len(fields)), Rows: [][]any{}}
	for i, f := range fields {
		res.Columns[i] = f.Name
	}
	for rr.NextRow() {
		if maxRows > 0 && len(res.Rows) == maxRows {
			res.Truncated = true
			continue
		}
		vals := rr.Values()
		row := make([]any, len(vals))
		for i, v := range vals {
			if v != nil {
				row[i] = string(v)
			}
		}
		res.Rows = append(res.Rows, row)
	}
	_, _ = rr.Close() // its error is the MultiResultReader's
	res.RowCount = len(res.Rows)
	return res
}

// Close ends the session with a Terminate, then closes the stream.
func (c *pgConn) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()
	return c.pg.Close(ctx)
}
