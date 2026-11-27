package pgconnector

import (
	"bytes"
	"encoding/hex"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

// item is a client message that waits for its reply from the database.
// The client side fills it before the database can see the message; then
// only the database side touches it, but copyIn.
type item struct {
	kind  byte // the client message type: Q, P, B, E, F, C, D, S
	log   bool // its end is a querylog line
	entry dbproxy.Entry
	start time.Time

	dataRows, tagRows, bytes int64        // DataRow count, CommandComplete rows, data bytes
	copyIn                   atomic.Int64 // COPY FROM bytes the client sent
	err                      string
}

// queue holds the items in the order the database answers them. The client
// side pushes an item before it writes the message, the database side pops.
type queue struct {
	mu    sync.Mutex
	items []*item
}

func (q *queue) push(it *item) {
	q.mu.Lock()
	q.items = append(q.items, it)
	q.mu.Unlock()
}

// peek is the first item, nil when none waits.
func (q *queue) peek() *item {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil
	}
	return q.items[0]
}

// pop drops the first item; the caller saw it through peek.
func (q *queue) pop() {
	q.mu.Lock()
	q.items[0] = nil
	q.items = q.items[1:]
	q.mu.Unlock()
}

// portal is a bound portal: its statement and logged parameters.
type portal struct {
	stmt, text string
	params     []string
}

// paramCap is the most of one value bindParams copies: a longer value is past
// dbproxy.MaxLogParams on its own, so LogParams cuts it anyway.
const paramCap = dbproxy.MaxLogParams + 1

// bindParams are the logged values of b's parameters: text as is, binary as
// \x<hex>, NULL as \N, cut by dbproxy.LogParams.
func bindParams(b *pgproto3.Bind) []string {
	if len(b.Parameters) == 0 {
		return nil
	}
	out := make([]string, len(b.Parameters))
	for i, p := range b.Parameters {
		switch {
		case p == nil:
			out[i] = `\N`
		case binaryParam(b.ParameterFormatCodes, i):
			out[i] = `\x` + hex.EncodeToString(p[:min(len(p), paramCap/2)])
		default:
			out[i] = string(p[:min(len(p), paramCap)])
		}
	}
	return dbproxy.LogParams(out)
}

// binaryParam reports whether parameter i is in binary format: no codes mean
// text, one code applies to all.
func binaryParam(formats []int16, i int) bool {
	if len(formats) == 1 {
		return formats[0] == 1
	}
	return i < len(formats) && formats[i] == 1
}

// tagRows is the row count that ends a CommandComplete tag ("SELECT 5",
// "INSERT 0 5", "COPY 10000"); 0 when the tag has none.
func tagRows(body []byte) int64 {
	tag := bytes.TrimRight(body, "\x00")
	var n int64
	for _, ch := range tag[bytes.LastIndexByte(tag, ' ')+1:] {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int64(ch-'0')
	}
	return n
}
