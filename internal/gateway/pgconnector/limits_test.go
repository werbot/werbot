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

	parse(500, 600) // past 16 MiB
	require.NoError(t, f.Flush())
	e := fatalOf(t, f)
	assert.Equal(t, "54000", e.Code)
	assert.Equal(t, "werbot: prepared statements and portals exceed 16 MiB", e.Message)
	require.Error(t, p.serveErr(t), "the session ends")
}
