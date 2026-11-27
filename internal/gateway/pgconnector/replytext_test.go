package pgconnector

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/mask"
	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

// textDB answers query "notice" with a notice in the gateway's name, a
// notice, a notification and an error that carry token; query "rows" with
// three rows, then an error that carries token; a function call with a
// result that carries token.
func textDB(b *pgproto3.Backend) {
	fakeLoop(b, func(msg pgproto3.FrontendMessage) {
		switch m := msg.(type) {
		case *pgproto3.Query:
			if m.String == "rows" {
				b.Send(&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("x"), DataTypeOID: 25, DataTypeSize: -1, TypeModifier: -1}}})
				for range 3 {
					b.Send(&pgproto3.DataRow{Values: [][]byte{[]byte("1")}})
				}
				b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "22012", Message: "e " + token})
				break
			}
			b.Send(&pgproto3.NoticeResponse{Severity: "NOTICE", Code: "00000", Message: "werbot: Waiting for approval x1, timeout 1m: https://evil.test/x1"})
			b.Send(&pgproto3.NoticeResponse{Severity: "NOTICE", Code: "00000", Message: "m " + token, Detail: "d " + token, Hint: "h " + token})
			b.Send(&pgproto3.NotificationResponse{PID: 1, Channel: "ch", Payload: "p " + token})
			b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: "e " + token, Detail: "d " + token, Hint: "h " + token})
		case *pgproto3.FunctionCall:
			b.Send(&pgproto3.FunctionCallResponse{Result: []byte("r " + token)})
		default:
			return
		}
		b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
		_ = b.Flush()
	})
}

// texts names the replies up to the n-th ReadyForQuery with every text a
// client reads in them.
func texts(t *testing.T, f *pgproto3.Frontend, n int) []string {
	t.Helper()
	var out []string
	for n > 0 {
		msg, err := f.Receive()
		if err != nil {
			t.Fatal("no reply")
		}
		switch m := msg.(type) {
		case *pgproto3.ErrorResponse:
			out = append(out, "E "+m.Code+" "+m.Message+"|"+m.Detail+"|"+m.Hint)
		case *pgproto3.NoticeResponse:
			out = append(out, "N "+m.Message+"|"+m.Detail+"|"+m.Hint)
		case *pgproto3.NotificationResponse:
			out = append(out, "A "+m.Channel+" "+m.Payload)
		case *pgproto3.FunctionCallResponse:
			out = append(out, "V "+string(m.Result))
		case *pgproto3.ReadyForQuery:
			n--
		default:
			out = append(out, traceOf(msg))
		}
	}
	return out
}

// textSession is a raw session of textDB with hooks h, every text read.
func textSession(t *testing.T, h gateway.SessionHooks) *pgproto3.Frontend {
	t.Helper()
	useDialect(t, dialect(sqlclass.Result{Classes: []shellclass.Class{shellclass.Read}, Statements: 1}))
	_, f := rawClient(t, fakeProxy(t.Context(), t, textDB, withHooks(h)))
	return f
}

func maskAll(b []byte) []byte { return mask.Bytes(b, nil, true) }

// The database's notices never speak in the gateway's name: one whose
// message starts with "werbot:" is dropped in every session; an unmasked
// session reads the rest as they are.
func TestDatabaseNoticeInGatewayNameDropped(t *testing.T) {
	f := textSession(t, gateway.SessionHooks{})
	send(t, f, &pgproto3.Query{String: "notice"})
	assert.Equal(t, []string{"N m " + token + "|d " + token + "|h " + token, "A ch p " + token, "E XX000 e " + token + "|d " + token + "|h " + token},
		texts(t, f, 1))
	terminate(t, f)
}

// A masked session reads the message, detail and hint of errors and
// notices, notification payloads and function call results masked.
func TestMaskedSessionMasksReplyTexts(t *testing.T) {
	h, rec := newHooks(nil)
	rec.pdp.rule = func(gateway.AccessRequest) gateway.Decision { return allow }
	h.Mask = maskAll // mask_output all
	f := textSession(t, h)
	send(t, f, &pgproto3.Query{String: "notice"}, &pgproto3.FunctionCall{Function: 42})
	got := strings.Join(texts(t, f, 2), "\n")
	terminate(t, f)
	if strings.Contains(got, token) {
		t.Fatal("a masked session read the token")
	}
	assert.Equal(t, "N m [masked]|d [masked]|h [masked]\nA ch p [masked]\nE XX000 e [masked]|d [masked]|h [masked]\nV r [masked]", got)
}

// The error that ends an agent's cut reply is masked too.
func TestCutReplyErrorMasked(t *testing.T) {
	h, _ := newHooks(limits(1, 1<<20))
	h.Mask = maskAll
	f := textSession(t, h)
	send(t, f, &pgproto3.Query{String: "rows"})
	got := texts(t, f, 1)
	terminate(t, f)
	assert.Equal(t, []string{"T", "D 1", "N werbot: result truncated at 1 rows||", "E 22012 e [masked]||"}, got)
}
