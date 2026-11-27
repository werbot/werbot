package pgconnector

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"strconv"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/gateway/recorder"
	"github.com/werbot/werbot/internal/platform/hashchain"
)

// chunkSink keeps the chunks of a recording.
type chunkSink struct {
	mu     sync.Mutex
	hashes [][]byte
	gz     [][]byte
}

func (s *chunkSink) Append(_ uuid.UUID, _ int, hash, gz []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hashes = append(s.hashes, bytes.Clone(hash))
	s.gz = append(s.gz, bytes.Clone(gz))
	return nil
}

func (*chunkSink) Close(uuid.UUID) error { return nil }

func TestQuerylogIntegrity(t *testing.T) {
	sink := &chunkSink{}
	rec := recorder.New(uuid.New(), 0, 0, sink, recorder.Options{Format: connector.RecordQuerylog})
	p := fakeProxy(t.Context(), t, func(b *pgproto3.Backend) {
		fakeLoop(b, func(msg pgproto3.FrontendMessage) {
			if _, ok := msg.(*pgproto3.Query); ok {
				b.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 0")})
				b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
				_ = b.Flush()
			}
		})
	}, func(p *proxy) { p.recording = rec })
	conn, err := pgconn.Connect(t.Context(), p.connectURL("main"))
	require.NoError(t, err)
	for i := range 100 {
		_, err := conn.Exec(t.Context(), "select "+strconv.Itoa(i)).ReadAll()
		require.NoError(t, err)
	}
	require.NoError(t, conn.Close(t.Context()))
	require.NoError(t, p.serveErr(t))
	require.NoError(t, rec.Close(t.Context()))

	prev := hashchain.Genesis()
	var raw []byte
	for i, gz := range sink.gz {
		prev = hashchain.Next(prev, gz)
		require.Equal(t, prev, sink.hashes[i], "chunk %d", i)
		zr, err := gzip.NewReader(bytes.NewReader(gz))
		require.NoError(t, err)
		b, err := io.ReadAll(zr)
		require.NoError(t, err)
		raw = append(raw, b...)
	}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	n := 0
	for ; sc.Scan(); n++ {
		var line struct{ Kind, SQL string }
		require.NoError(t, json.Unmarshal(sc.Bytes(), &line))
		assert.Equal(t, "query", line.Kind)
		assert.Equal(t, "select "+strconv.Itoa(n), line.SQL)
	}
	assert.Equal(t, 100, n)
}

func TestBindParamsUnambiguous(t *testing.T) {
	b := &pgproto3.Bind{ParameterFormatCodes: []int16{0, 0, 1, 0, 0, 0},
		Parameters: [][]byte{nil, []byte(`\N`), {0x41}, []byte(`\x41`), []byte("plain"), {}}}
	got := bindParams(b)
	assert.Equal(t, []string{`\N`, `\\N`, `\x41`, `\\x41`, "plain", ""}, got)
	seen := map[string]bool{}
	for _, v := range got {
		assert.False(t, seen[v], "%q displays like another value", v)
		seen[v] = true
	}
}
