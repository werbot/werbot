package pgconnector

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetainedBytesLimit(t *testing.T) {
	p := fakeProxy(t.Context(), t, func(b *pgproto3.Backend) {
		fakeLoop(b, func(msg pgproto3.FrontendMessage) {
			if _, ok := msg.(*pgproto3.Sync); ok {
				b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
				_ = b.Flush()
			}
		})
	})
	conn, f := rawClient(t, p)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	text := strings.Repeat("x", 60<<10)
	parse := func(from, to int) {
		for i := from; i < to; i++ {
			f.Send(&pgproto3.Parse{Name: "s" + strconv.Itoa(i), Query: text})
		}
	}
	parse(0, 250) // about 15 MiB kept
	for i := range 250 {
		f.Send(&pgproto3.Close{ObjectType: 'S', Name: "s" + strconv.Itoa(i)})
	}
	parse(250, 500)
	f.Send(&pgproto3.Sync{})
	require.NoError(t, f.Flush())
	untilReady(t, f) // a Close gives back what its statement kept

	// up to the first Parse past 16 MiB: the session ends at it, so the client
	// writes nothing after and reads the FATAL
	size := len("s500") + len(text)
	parse(500, 500+(maxRetained-250*size)/size+1)
	require.NoError(t, f.Flush())
	e := fatalOf(t, f)
	assert.Equal(t, "54000", e.Code)
	assert.Equal(t, "werbot: prepared statements and portals exceed 16 MiB", e.Message)
	require.ErrorIs(t, p.serveErr(t), errRetained, "the session ends")
}

// Messages that wait for a reply are bounded per session: one past
// maxPending ends it with a FATAL.
func TestPendingQueueLimit(t *testing.T) {
	p := fakeProxy(t.Context(), t, func(b *pgproto3.Backend) { fakeLoop(b, func(pgproto3.FrontendMessage) {}) }) // never answers
	conn, f := rawClient(t, p)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	for range maxPending + 1 {
		f.Send(&pgproto3.Sync{})
	}
	require.NoError(t, f.Flush())
	e := fatalOf(t, f)
	assert.Equal(t, "54000", e.Code)
	assert.Equal(t, "werbot: too many pending messages", e.Message)
	require.ErrorIs(t, p.serveErr(t), errPending, "the session ends")
}

// A Bind's parameters are kept with its portal for an approval of its
// execute, within the retained bytes; they are logged only with record_params.
func TestBindParamsOnlyWhenRecorded(t *testing.T) {
	body, err := (&pgproto3.Bind{DestinationPortal: "p", Parameters: [][]byte{[]byte("v1"), nil}}).Encode(nil)
	require.NoError(t, err)
	shown := []string{"v1", `\N`}
	for _, record := range []bool{false, true} {
		c := &clientSide{log: sessionLog{params: record}, stmts: map[string]named{}, portals: map[string]named{}}
		it, err := c.op('B', body[5:])
		require.NoError(t, err)
		logged := []string(nil)
		if record {
			logged = shown
		}
		assert.Equal(t, shown, c.portals["p"].params, "record_params %v", record)
		assert.Equal(t, logged, it.entry.Params, "record_params %v", record)
		assert.Equal(t, len("p")+len(strings.Join(shown, "")), c.kept, "record_params %v", record)
	}
}
