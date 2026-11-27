package pgconnector

import (
	"bufio"
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

type encoder interface{ Encode([]byte) ([]byte, error) }

func encodeAll(f *testing.F, msgs ...encoder) []byte {
	var b []byte
	for _, m := range msgs {
		var err error
		if b, err = m.Encode(b); err != nil {
			f.Fatal(err)
		}
	}
	return b
}

// FuzzPGMessage feeds arbitrary client and database streams through the
// start parser and both sides of the protocol relay: no input panics.
func FuzzPGMessage(f *testing.F) {
	client := encodeAll(f,
		&pgproto3.Query{String: "select 1"},
		&pgproto3.Parse{Name: "s1", Query: "select $1::int", ParameterOIDs: []uint32{23}},
		&pgproto3.Bind{DestinationPortal: "p1", PreparedStatement: "s1", ParameterFormatCodes: []int16{1},
			Parameters: [][]byte{{0, 0, 0, 42}, nil}, ResultFormatCodes: []int16{0}},
		&pgproto3.Describe{ObjectType: 'P', Name: "p1"},
		&pgproto3.Execute{Portal: "p1"},
		&pgproto3.Sync{},
		&pgproto3.FunctionCall{Function: 123, Arguments: [][]byte{[]byte("x")}},
		&pgproto3.CopyData{Data: []byte("1\tx\n")},
		&pgproto3.CopyDone{},
		&pgproto3.Close{ObjectType: 'S', Name: "s1"},
		&pgproto3.Terminate{},
	)
	db := encodeAll(f,
		&pgproto3.ParseComplete{}, &pgproto3.BindComplete{},
		&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("x"), DataTypeOID: 23}}},
		&pgproto3.DataRow{Values: [][]byte{[]byte("1")}},
		&pgproto3.CommandComplete{CommandTag: []byte("SELECT 1")},
		&pgproto3.CopyInResponse{}, &pgproto3.CopyData{Data: []byte("x")},
		&pgproto3.ErrorResponse{Severity: "ERROR", Code: "22012", Message: "division by zero"},
		&pgproto3.ReadyForQuery{TxStatus: 'I'},
	)
	start := encodeAll(f, &pgproto3.SSLRequest{},
		&pgproto3.StartupMessage{ProtocolVersion: pgproto3.ProtocolVersion30, Parameters: map[string]string{"user": "u", "database": "d"}})
	f.Add(client, db)
	f.Add(start, []byte{})
	f.Add([]byte{'Q', 0, 0, 0, 2}, []byte{'Z', 0xff, 0xff, 0xff, 0xff})
	f.Add([]byte{'B', 0x01, 0, 0, 5, 'x'}, db[:7])
	f.Fuzz(func(t *testing.T, client, db []byte) {
		peek := func(n int) ([]byte, error) {
			if n > len(client) {
				return nil, io.EOF
			}
			return client[:n], nil
		}
		if p, err := nextPacket(peek, func() error { return nil }); err == nil {
			_, _ = startupMessage(peek, p)
		}
		g := dbproxy.NewGuard(&gateway.Session{Recording: &memRecording{}}, dbproxy.GuardConfig{Type: "postgres"})
		defer g.Close()
		dbConn := struct {
			io.Reader
			io.Writer
		}{bytes.NewReader(db), io.Discard}
		c, d := newRelay(t.Context(), bufio.NewReaderSize(bytes.NewReader(client), ioBuf), dbConn, io.Discard, g,
			sessionLog{t0: time.Now(), db: "d", params: true})
		_ = run(c.next)
		_ = run(d.next)
	})
}
