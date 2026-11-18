package dbx

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// errOneQueryRow is any call on a Read but its one QueryRow.
var errOneQueryRow = errors.New("dbx: ReadInOrg runs exactly one QueryRow")

// setContext is the organization context of InOrg as one statement.
const setContext = "SELECT set_config('werbot.org_id', $1, true), set_config('werbot.user_id', $2, true)"

// ReadInOrg runs the one statement of fn like InOrg, in one round trip: BEGIN,
// the organization context, the statement and COMMIT go as one pipelined
// batch, under the context of the QueryRow call. fn makes exactly one QueryRow
// call on r (a sqlc DBTX) and scans it; the statement's errors, pgx.ErrNoRows
// included, come from Scan as in a Tx. A failed statement rolls back. Use it
// only for a read of one statement.
func ReadInOrg(pool *pgxpool.Pool, org, user uuid.UUID, fn func(r *Read) error) error {
	if org == uuid.Nil {
		return ErrNoOrg
	}
	r := &Read{pool: pool, org: org, user: user}
	err := fn(r)
	if ferr := r.finish(); err == nil {
		err = ferr
	}
	return err
}

// Read is the statement of a ReadInOrg; only QueryRow works, once.
type Read struct {
	pool      *pgxpool.Pool
	org, user uuid.UUID
	ctx       context.Context // of the one QueryRow, also for its ROLLBACK
	conn      *pgxpool.Conn   // set while the batch is open
	br        pgx.BatchResults
	sent      bool
}

// QueryRow sends the batch with sql as its statement.
func (r *Read) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if r.sent {
		return errRow{errOneQueryRow}
	}
	r.sent = true
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return errRow{err}
	}
	b := &pgx.Batch{}
	b.Queue("BEGIN")
	b.Queue(setContext, r.org.String(), r.user.String())
	b.Queue(sql, args...)
	b.Queue("COMMIT")
	r.ctx, r.conn, r.br = ctx, conn, conn.SendBatch(ctx, b)
	return row{r}
}

// Exec is not supported: a Read runs one QueryRow.
func (*Read) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errOneQueryRow
}

// Query is not supported: a Read runs one QueryRow.
func (*Read) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, errOneQueryRow }

// finish reads what is left of the batch (COMMIT), rolls back a transaction a
// failed statement left open and returns the connection. It runs once.
func (r *Read) finish() error {
	if r.conn == nil {
		return nil
	}
	conn := r.conn
	r.conn = nil
	defer conn.Release()
	err := r.br.Close()
	if conn.Conn().PgConn().TxStatus() != 'I' && !conn.Conn().IsClosed() {
		// a failed statement skips COMMIT; Release would close a connection in a transaction
		if _, rerr := conn.Exec(r.ctx, "ROLLBACK"); err == nil {
			err = rerr
		}
	}
	return err
}

// row scans the statement's result: BEGIN and the context come first.
type row struct{ r *Read }

func (w row) Scan(dest ...any) error {
	r := w.r
	if r.conn == nil {
		return errOneQueryRow
	}
	var err error
	for range 2 {
		if _, err = r.br.Exec(); err != nil {
			break
		}
	}
	if err == nil {
		err = r.br.QueryRow().Scan(dest...)
	}
	if ferr := r.finish(); err == nil {
		err = ferr
	}
	return err
}

type errRow struct{ err error }

func (e errRow) Scan(...any) error { return e.err }
