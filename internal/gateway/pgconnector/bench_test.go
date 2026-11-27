package pgconnector

import (
	"context"
	"net"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/gateway"
)

// directURL connects to the test database itself, with the resource credential.
func directURL(cfg *postgresv1.Config, secret gateway.Secret) string {
	return "postgres://" + cfg.GetUsername() + ":" + string(secret.Value) + "@" +
		net.JoinHostPort(cfg.GetHost(), strconv.Itoa(int(cfg.GetPort()))) + "/" + cfg.GetDatabase() + "?sslmode=disable"
}

// pct is the q-th percentile of ds.
func pct(ds []time.Duration, q int) time.Duration {
	s := slices.Clone(ds)
	slices.Sort(s)
	return s[(len(s)*q)/100]
}

// BenchmarkPostgresPassthrough runs the same operation directly and through
// the connector, interleaved, and reports what the connector adds. Budgets
// (gated by TestHotPathBudget, not here): simple and prepared add p50 ≤ 0.1 ms
// and p99 ≤ 0.5 ms; stream through the connector ≥ 0.9 × direct bytes/s.
func BenchmarkPostgresPassthrough(b *testing.B) {
	cfg, secret := realPG(b)
	conns := pair(b, cfg, secret, serveProxy(b.Context(), b, testResource(b, cfg), secret))
	b.Run("simple", func(b *testing.B) {
		latency(b, conns, func(c *pgconn.PgConn) error { _, err := c.Exec(b.Context(), "select 1").ReadAll(); return err })
	})
	b.Run("prepared", func(b *testing.B) {
		latency(b, conns, func(c *pgconn.PgConn) error {
			_, err := c.ExecPrepared(b.Context(), "bench", [][]byte{[]byte("42")}, nil, nil).Close()
			return err
		})
	})
	b.Run("stream", func(b *testing.B) { stream(b, conns, streamRows, streamRowBytes) })
}

// pair connects to the test database directly and through p, in that order,
// and prepares "bench" (select $1::int) on both.
func pair(tb testing.TB, cfg *postgresv1.Config, secret gateway.Secret, p *proxy) [2]*pgconn.PgConn {
	tb.Helper()
	conns := [2]*pgconn.PgConn{}
	for i, url := range []string{directURL(cfg, secret), p.connectURL(cfg.GetDatabase())} {
		c, err := pgconn.Connect(tb.Context(), url)
		if err != nil {
			tb.Fatal("connect failed")
		}
		tb.Cleanup(func() { _ = c.Close(context.Background()) })
		if _, err := c.Prepare(tb.Context(), "bench", "select $1::int", nil); err != nil {
			tb.Fatal("prepare failed")
		}
		conns[i] = c
	}
	return conns
}

// timed is the duration of op on c; a failed op fails the test or benchmark.
func timed(tb testing.TB, c *pgconn.PgConn, op func(*pgconn.PgConn) error) time.Duration {
	t0 := time.Now()
	if err := op(c); err != nil {
		tb.Fatal("operation failed")
	}
	return time.Since(t0)
}

func latency(b *testing.B, conns [2]*pgconn.PgConn, op func(*pgconn.PgConn) error) {
	var d, v []time.Duration
	for b.Loop() {
		d = append(d, timed(b, conns[0], op))
		v = append(v, timed(b, conns[1], op))
	}
	p50, p99 := pct(v, 50)-pct(d, 50), pct(v, 99)-pct(d, 99)
	b.ReportMetric(float64(p50.Microseconds())/1000, "added-p50-ms")
	b.ReportMetric(float64(p99.Microseconds())/1000, "added-p99-ms")
}

// The default stream payload: 8 MiB of 1 KiB rows.
const (
	streamRows     = 8192
	streamRowBytes = 1024
)

// streamQuery returns rows rows of rowBytes bytes each.
func streamQuery(rows, rowBytes int) string {
	return "select repeat('x', " + strconv.Itoa(rowBytes) + ") from generate_series(1, " + strconv.Itoa(rows) + ")"
}

// streamOp reads every row of streamQuery(rows, rowBytes) and drops it.
func streamOp(ctx context.Context, rows, rowBytes int) func(*pgconn.PgConn) error {
	sql := streamQuery(rows, rowBytes)
	return func(c *pgconn.PgConn) error {
		rr := c.ExecParams(ctx, sql, nil, nil, nil, nil)
		for rr.NextRow() {
		}
		_, err := rr.Close()
		return err
	}
}

// stream reads a payload of rows × rowBytes both ways and reports the
// connector's bytes/s and its ratio to direct.
func stream(b *testing.B, conns [2]*pgconn.PgConn, rows, rowBytes int) {
	op := streamOp(b.Context(), rows, rowBytes)
	timed(b, conns[0], op) // warm-up
	timed(b, conns[1], op)
	var d, v time.Duration
	for b.Loop() {
		d += timed(b, conns[0], op)
		v += timed(b, conns[1], op)
	}
	b.ReportMetric(float64(rows*rowBytes)*float64(b.N)/v.Seconds()/1e6, "proxy-MB/s")
	b.ReportMetric(d.Seconds()/v.Seconds(), "proxy/direct")
}
