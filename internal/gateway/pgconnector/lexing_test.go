package pgconnector

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"

	"github.com/werbot/werbot/internal/gateway"
)

const setOff = "set standard_conforming_strings = off"

// lexingDB answers like a database whose standard_conforming_strings
// setOff turns off: the ParameterStatus comes before the ReadyForQuery
// that ends the cycle, and the reply to a simple setOff comes late.
func lexingDB(b *pgproto3.Backend) {
	changed := false
	fakeLoop(b, func(msg pgproto3.FrontendMessage) {
		switch m := msg.(type) {
		case *pgproto3.Query:
			if m.String == setOff {
				time.Sleep(100 * time.Millisecond) // a pipelined query is at the gateway by now
				changed = true
			}
			b.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 0")})
		case *pgproto3.Parse:
			changed = changed || m.Query == setOff
			b.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Bind:
			b.Send(&pgproto3.BindComplete{})
		case *pgproto3.Execute:
			b.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 0")})
		}
		switch msg.(type) {
		case *pgproto3.Query, *pgproto3.Sync:
			if changed {
				b.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "off"})
			}
			b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
		}
		_ = b.Flush()
	})
}

// A lexing setting a person's allowed operation may change makes the later
// operations of its cycle opaque, and the next cycle waits for the reply
// that reports the setting, so no text is classified under the old lexing.
func TestPipelinedLexingChangeIsOpaque(t *testing.T) {
	warm(t)
	for name, tc := range map[string]struct {
		msgs  []pgproto3.FrontendMessage
		ready int // ReadyForQuery replies to msgs
		want  [][]string
	}{
		"simple queries": {
			[]pgproto3.FrontendMessage{&pgproto3.Query{String: setOff}, &pgproto3.Query{String: "select 1"}}, 2,
			[][]string{{"privilege"}, {"opaque"}, {"opaque"}},
		},
		"one extended batch": {
			[]pgproto3.FrontendMessage{&pgproto3.Parse{Query: setOff}, &pgproto3.Bind{}, &pgproto3.Execute{},
				&pgproto3.Parse{Query: "select 1"}, &pgproto3.Bind{}, &pgproto3.Execute{}, &pgproto3.Sync{}}, 1,
			[][]string{{"privilege"}, {"opaque"}, {"opaque"}},
		},
		"a setting that keeps the lexing": {
			[]pgproto3.FrontendMessage{&pgproto3.Query{String: "set search_path = app"}, &pgproto3.Query{String: "select 1"}}, 2,
			[][]string{{"privilege"}, {"read"}, {"read"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			h, rec := newHooks(nil)
			rec.pdp.rule = func(gateway.AccessRequest) gateway.Decision { return allow }
			p := fakeProxy(t.Context(), t, lexingDB, withHooks(h))
			_, f := rawClient(t, p)
			send(t, f, tc.msgs...)
			trace(t, f, tc.ready)
			send(t, f, &pgproto3.Query{String: "select 1"})
			trace(t, f, 1)
			terminate(t, f)
			var got [][]string
			for _, e := range p.entries(t) {
				got = append(got, e.Classes)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}
