package pgconnector

import (
	"crypto/sha256"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

const update = "UPDATE t SET a = 2 WHERE id = 1"

// agentProxy serves agent sessions of cfg with the read-only secret ro; the
// approved writes get rw.
func agentProxy(t *testing.T, cfg *postgresv1.Config, ro, rw gateway.Secret) (*proxy, *hookRec) {
	t.Helper()
	h, rec := newHooks(agentLimits())
	rec.cred = rw
	return serveProxy(t.Context(), t, testResource(t, cfg), ro, withHooks(h)), rec
}

func TestAgentApprovedWriteUsesPrimary(t *testing.T) {
	warm(t)
	cfg, ro, rw, owner := agentDB(t)
	p, rec := agentProxy(t, cfg, ro, rw)
	var notices []string
	conn := clientOf(t, p, cfg.GetDatabase(), &notices, nil)
	rs, err := conn.Exec(t.Context(), update).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, "UPDATE 1", rs[0].CommandTag.String())
	assert.Equal(t, byte('I'), conn.TxStatus())
	assert.Equal(t, []string{waitingNotice}, notices)
	assert.Equal(t, "2", value(t, owner, "select a from t"), "the write ran as the resource's username")
	_, writes := rec.asked()
	assert.Equal(t, []writeCall{{approvalID, sha256.Sum256([]byte(update))}}, writes, "the credential is bound to the approval and the text")
	const text = "UPDATE t SET a = $1 WHERE id = $2"
	res := conn.ExecParams(t.Context(), text, [][]byte{[]byte("5"), []byte("1")}, nil, nil, nil).Read()
	require.NoError(t, res.Err)
	assert.Equal(t, "UPDATE 1", res.CommandTag.String())
	assert.Equal(t, "5", value(t, owner, "select a from t"))
	_, writes = rec.asked()
	require.Len(t, writes, 2)
	assert.Equal(t, sha256.Sum256([]byte(text+"\n-- params: [\"5\",\"1\"]")), writes[1].sha, "the credential is bound to the text and its parameters")
	assert.Equal(t, cfg.GetAgentUsername(), value(t, conn, "select current_user"), "the agent's own session stays read-only")
	assert.Equal(t, "on", value(t, conn, "show transaction_read_only"))
	require.NoError(t, conn.Close(t.Context()))
	assert.Contains(t, lines(p.entries(t)), logLine{kind: "query", text: update, rows: 1})
}

func TestAgentWriteDeniedInTransaction(t *testing.T) {
	warm(t)
	cfg, ro, rw, owner := agentDB(t)
	p, rec := agentProxy(t, cfg, ro, rw)
	conn := clientOf(t, p, cfg.GetDatabase(), nil, nil)
	_, err := conn.Exec(t.Context(), "BEGIN").ReadAll()
	require.NoError(t, err)
	_, err = conn.Exec(t.Context(), update).ReadAll()
	pe := pgErr(t, err)
	assert.Equal(t, "42501", pe.Code)
	assert.Equal(t, "werbot: query denied: approved writes run outside a transaction", pe.Message)
	assert.Equal(t, byte('T'), conn.TxStatus(), "the transaction stays open")
	approvals, writes := rec.asked()
	assert.Empty(t, approvals, "no approval is requested")
	assert.Empty(t, writes)
	assert.Equal(t, "1", value(t, owner, "select a from t"))
}

func TestAgentWriteSingleStatement(t *testing.T) {
	warm(t)
	cfg, ro, rw, owner := agentDB(t)
	p, rec := agentProxy(t, cfg, ro, rw)
	conn := clientOf(t, p, cfg.GetDatabase(), nil, nil)
	_, err := conn.Exec(t.Context(), update+"; UPDATE t SET a = 3 WHERE id = 1").ReadAll()
	pe := pgErr(t, err)
	assert.Equal(t, "werbot: query denied: approved writes run one statement at a time", pe.Message)

	_, err = conn.Prepare(t.Context(), "w", update, nil) // Parse without an Execute
	pe = pgErr(t, err)
	assert.Equal(t, "42501", pe.Code)
	assert.Equal(t, "werbot: query denied: approved writes run one statement at a time", pe.Message)

	assert.Equal(t, "1", value(t, conn, "select a from t"), "the session lives")
	approvals, _ := rec.asked()
	assert.Empty(t, approvals)
	assert.Equal(t, "1", value(t, owner, "select a from t"))
}

func TestAgentWriteCredentialRefused(t *testing.T) {
	warm(t)
	cfg, ro, rw, owner := agentDB(t)
	p, rec := agentProxy(t, cfg, ro, rw)
	rec.credErr = errors.New("permission denied: write not approved")
	conn := clientOf(t, p, cfg.GetDatabase(), nil, nil)
	_, err := conn.Exec(t.Context(), update).ReadAll()
	pe := pgErr(t, err)
	assert.Equal(t, "42501", pe.Code)
	assert.Equal(t, "werbot: query denied: write not approved", pe.Message)
	assert.Equal(t, "1", value(t, owner, "select a from t"))
	assert.Equal(t, "1", value(t, conn, "select a from t"), "the session lives")
}

func TestAgentStartupParamsFiltered(t *testing.T) {
	warm(t)
	cfg, ro, rw, _ := agentDB(t)
	p, _ := agentProxy(t, cfg, ro, rw)
	params := map[string]string{"application_name": "agent-app", "options": "-c statement_timeout=0 -c default_transaction_read_only=off",
		"statement_timeout": "0", "default_transaction_read_only": "off", "role": cfg.GetUsername(), "search_path": "evil"}
	conn := clientOf(t, p, cfg.GetDatabase(), nil, params)
	assert.Equal(t, "30s", value(t, conn, "show statement_timeout"))
	assert.Equal(t, "on", value(t, conn, "show default_transaction_read_only"))
	assert.Equal(t, "agent-app", value(t, conn, "show application_name"))
	assert.Equal(t, cfg.GetAgentUsername(), value(t, conn, "select current_user"))
	assert.NotEqual(t, "evil", value(t, conn, "show search_path"))

	person := clientOf(t, serveProxy(t.Context(), t, testResource(t, cfg), rw), cfg.GetDatabase(), nil,
		map[string]string{"options": "-c search_path=app", "standard_conforming_strings": "off"})
	assert.Equal(t, "app", value(t, person, "show search_path"), "a person's parameters pass as in stage 17")
	assert.Equal(t, "on", value(t, person, "show standard_conforming_strings"), "but the lexing settings")

	assert.Equal(t, map[string]string{"application_name": "a", "DateStyle": "ISO", "statement_timeout": "1500",
		"standard_conforming_strings": "on", "default_transaction_read_only": "on"},
		agentStartupParams(map[string]string{"application_name": "a", "DateStyle": "ISO", "options": "-c role=x", "backslash_quote": "on"},
			gateway.AgentLimits{StatementTimeout: 1500 * time.Millisecond}))
}

func TestApprovedWriteDialsThroughSession(t *testing.T) {
	warm(t)
	type start struct {
		params   map[string]string
		password string
	}
	starts := make(chan start, 4)
	f := newFakeDB(t, nil, func(b *pgproto3.Backend, _ net.Conn, sm *pgproto3.StartupMessage) {
		b.Send(&pgproto3.AuthenticationCleartextPassword{})
		if b.Flush() != nil || b.SetAuthType(pgproto3.AuthTypeCleartextPassword) != nil {
			return
		}
		msg, err := b.Receive()
		pw, ok := msg.(*pgproto3.PasswordMessage)
		if err != nil || !ok {
			return
		}
		starts <- start{sm.Parameters, pw.Password}
		if !fakeLogin(b) {
			return
		}
		fakeLoop(b, func(pgproto3.FrontendMessage) {
			b.Send(&pgproto3.CommandComplete{CommandTag: []byte("UPDATE 1")})
			b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
			_ = b.Flush()
		})
	})
	cfg := &postgresv1.Config{Host: "127.0.0.1", Port: f.port, Database: "main", Username: "app", AgentUsername: "agent", Sslmode: "disable"}
	p, _ := agentProxy(t, cfg, passwordSecret("ro-secret"), passwordSecret("rw-secret"))
	conn := clientOf(t, p, "main", nil, map[string]string{"application_name": "agent-app", "options": "-c role=app"})
	session := <-starts
	_, err := conn.Exec(t.Context(), update).ReadAll()
	require.NoError(t, err)
	write := <-starts
	assert.Equal(t, int32(2), p.dials.Load(), "the write dials through the session's dialer")
	assert.Equal(t, "agent", session.params["user"])
	assert.Equal(t, "app", write.params["user"], "the write logs in as the resource's username")
	if session.password != "ro-secret" || write.password != "rw-secret" {
		t.Fatal("the session or the write logged in with the wrong credential")
	}
	assert.Equal(t, "on", session.params["default_transaction_read_only"])
	assert.NotContains(t, write.params, "default_transaction_read_only")
	assert.NotContains(t, write.params, "options")
	for _, k := range []string{"statement_timeout", "standard_conforming_strings", "application_name"} {
		assert.Equal(t, session.params[k], write.params[k], k)
	}
}

func TestAgentClassifierFailureStillReadOnly(t *testing.T) {
	useDialect(t, dialect(sqlclass.Result{Classes: []shellclass.Class{shellclass.Read}, Statements: 1}))
	cfg, ro, rw, owner := agentDB(t)
	p, rec := agentProxy(t, cfg, ro, rw)
	conn := clientOf(t, p, cfg.GetDatabase(), nil, nil)
	_, err := conn.Exec(t.Context(), "SET default_transaction_read_only = off /* classifier failure */").ReadAll()
	require.NoError(t, err, "a classifier that fails open lets the statement through")
	_, err = conn.Exec(t.Context(), "UPDATE t SET a = 2 WHERE id = 1 /* classifier failure */").ReadAll()
	pe := pgErr(t, err)
	assert.Equal(t, "42501", pe.Code, "the database refuses the read-only role")
	assert.Contains(t, pe.Message, "permission denied")
	assert.Equal(t, "1", value(t, owner, "select a from t"))
	approvals, writes := rec.asked()
	assert.Empty(t, approvals)
	assert.Empty(t, writes)
}
