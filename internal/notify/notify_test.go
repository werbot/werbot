package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/jobs"
	"github.com/werbot/werbot/internal/platform/secretbox"
)

type sent struct{ to, subject, body string }

type fakeMailer struct {
	mu   sync.Mutex
	sent []sent
}

func (f *fakeMailer) Send(_ context.Context, to, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sent{to, subject, body})
	return nil
}

func newBox(t *testing.T, b byte) *secretbox.Box {
	t.Helper()
	box, err := secretbox.New(bytes.Repeat([]byte{b}, 32), nil)
	require.NoError(t, err)
	return box
}

func TestRenderEscapesData(t *testing.T) {
	subj, html, err := Render("invite", map[string]string{"url": "https://w.example/invite/abc", "org": "Acme <b>", "inviter": "Bob"})
	require.NoError(t, err)
	assert.Equal(t, "Bob invited you to Acme <b> on Werbot", subj)
	assert.Contains(t, html, `href="https://w.example/invite/abc"`)
	assert.Contains(t, html, "Acme &lt;b&gt;")
}

func TestRenderSubjectStripsLineBreaks(t *testing.T) {
	subj, _, err := Render("invite", map[string]string{"url": "u", "org": "A\r\nBcc: x", "inviter": "Bob"})
	require.NoError(t, err)
	assert.NotContains(t, subj, "\n")
	assert.NotContains(t, subj, "\r")
}

func TestRenderOtherTemplates(t *testing.T) {
	data := map[string]string{"url": "https://w.example/x", "name": "Ann"}
	subj, html, err := Render("password_reset", data)
	require.NoError(t, err)
	assert.Equal(t, "Reset your Werbot password", subj)
	assert.Contains(t, html, `href="https://w.example/x"`)
	subj, html, err = Render("account_deletion", data)
	require.NoError(t, err)
	assert.Equal(t, "Confirm deletion of your Werbot account", subj)
	assert.Contains(t, html, `href="https://w.example/x"`)
}

func TestRenderAccessTemplates(t *testing.T) {
	for _, tc := range []struct {
		name, subject string
		data          map[string]string
		body          []string
	}{
		{"access_request", "bob requests access to db <1>",
			map[string]string{"url": "https://w.example/requests?id=1", "requester": "bob", "resource": "db <1>",
				"duration": "2h", "reason": "fix <b>", "command": "rm -rf /tmp/<x>"},
			[]string{`href="https://w.example/requests?id=1"`, "db &lt;1&gt;", "2h", "fix &lt;b&gt;", "rm -rf /tmp/&lt;x&gt;"}},
		{"access_decision", "Your request for env=prod was approved",
			map[string]string{"url": "https://w.example/requests?id=2", "resource": "env=prod", "status": "approved", "comment": "ok <i>"},
			[]string{`href="https://w.example/requests?id=2"`, "approved", "ok &lt;i&gt;"}},
		{"break_glass", "Break-glass access to db by bob",
			map[string]string{"url": "https://w.example/requests?id=3", "requester": "bob", "resource": "db", "reason": "incident <1>"},
			[]string{`href="https://w.example/requests?id=3"`, "bob", "incident &lt;1&gt;"}},
	} {
		subj, html, err := Render(tc.name, tc.data)
		require.NoError(t, err, tc.name)
		assert.Equal(t, tc.subject, subj, tc.name)
		for _, b := range tc.body {
			assert.Contains(t, html, b, tc.name)
		}
	}
	_, html, err := Render("access_request", map[string]string{"url": "u", "requester": "bob", "resource": "db",
		"duration": "2h", "reason": "", "command": ""})
	require.NoError(t, err)
	assert.NotContains(t, html, "Command", "an access request shows no command block")
	subj, _, err := Render("access_decision", map[string]string{"url": "u", "resource": "a\r\nBcc: x", "status": "denied", "comment": ""})
	require.NoError(t, err)
	assert.NotContains(t, subj, "\n")
	assert.NotContains(t, subj, "\r")
}

func TestRenderAgentAlert(t *testing.T) {
	subj, html, err := Render("agent_alert", map[string]string{"url": "https://w.example/agents/runs/1", "agent": "claude",
		"sponsor": "bob", "resource": "db", "command": "cat <id_rsa> && echo \"x\"", "classes": "read, secrets"})
	require.NoError(t, err)
	assert.Equal(t, "Werbot: agent claude attempted read, secrets on db", subj)
	for _, b := range []string{`href="https://w.example/agents/runs/1"`, "bob", "db", "read, secrets",
		"cat &lt;id_rsa&gt; &amp;&amp; echo &#34;x&#34;"} {
		assert.Contains(t, html, b)
	}
	assert.NotContains(t, html, "<id_rsa>", "the command is escaped")
	subj, _, err = Render("agent_alert", map[string]string{"url": "u", "agent": "claude", "sponsor": "bob",
		"resource": "db\r\nBcc: x", "command": "id", "classes": "egress"})
	require.NoError(t, err)
	assert.NotContains(t, subj, "\n")
}

func TestRenderUnknownTemplate(t *testing.T) {
	_, _, err := Render("nope", nil)
	assert.ErrorContains(t, err, "unknown template")
}

func sealed(t *testing.T, box *secretbox.Box, data map[string]string) SendEmailArgs {
	t.Helper()
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	ct, kid, err := box.Seal(raw)
	require.NoError(t, err)
	return SendEmailArgs{To: "a@example.com", Template: "password_reset", SealedData: ct, KeyID: kid}
}

func TestWorkerSends(t *testing.T) {
	box := newBox(t, 1)
	m := &fakeMailer{}
	args := sealed(t, box, map[string]string{"url": "https://w.example/reset/xyz", "name": "Ann"})
	require.NoError(t, NewWorker(m, box).Work(t.Context(), &river.Job[SendEmailArgs]{Args: args}))
	require.Len(t, m.sent, 1)
	assert.Equal(t, "a@example.com", m.sent[0].to)
	assert.Equal(t, "Reset your Werbot password", m.sent[0].subject)
	assert.Contains(t, m.sent[0].body, "https://w.example/reset/xyz")
}

func TestWorkerCancelsOnUnknownKey(t *testing.T) {
	m := &fakeMailer{}
	args := sealed(t, newBox(t, 2), map[string]string{"url": "u", "name": "n"})
	err := NewWorker(m, newBox(t, 1)).Work(t.Context(), &river.Job[SendEmailArgs]{Args: args})
	var cancel *river.JobCancelError
	require.True(t, errors.As(err, &cancel), "got %v", err)
	assert.Empty(t, m.sent)
}

func TestQueueEnqueueTx(t *testing.T) {
	pool, _ := dbtest.New(t)
	box := newBox(t, 1)
	workers := river.NewWorkers()
	river.AddWorker(workers, NewWorker(&fakeMailer{}, box))
	client, err := jobs.NewClient(pool, workers, nil, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	q := NewQueue(client, box)

	token := "tok_" + strings.Repeat("a", 39)
	data := map[string]string{"url": "https://w.example/invite/" + token, "org": "Acme", "inviter": "Bob"}
	ctx := t.Context()
	require.NoError(t, dbx.InOrg(ctx, pool, uuid.New(), uuid.Nil, func(tx pgx.Tx) error {
		require.NoError(t, q.EnqueueTx(ctx, tx, "a@example.com", "invite", data))
		job := rivertest.RequireInsertedTx[*riverpgxv5.Driver](ctx, t, tx, &SendEmailArgs{}, &rivertest.RequireInsertedOpts{Queue: jobs.QueueEmail, MaxAttempts: 5})
		assert.Equal(t, "email", job.Queue)
		assert.Equal(t, "a@example.com", job.Args.To)
		assert.Equal(t, "invite", job.Args.Template)
		assert.Equal(t, box.KeyID(), job.Args.KeyID)
		plain, err := box.Open(job.Args.SealedData, job.Args.KeyID)
		require.NoError(t, err)
		var got map[string]string
		require.NoError(t, json.Unmarshal(plain, &got))
		assert.Equal(t, data, got)

		var args string
		require.NoError(t, tx.QueryRow(ctx, "SELECT args::text FROM river_job WHERE kind = 'send_email'").Scan(&args))
		assert.NotContains(t, args, token)
		return nil
	}))

	err = dbx.InOrg(ctx, pool, uuid.New(), uuid.Nil, func(tx pgx.Tx) error {
		return q.EnqueueTx(ctx, tx, "a@example.com", "nope", nil)
	})
	assert.ErrorContains(t, err, "unknown template")
}

func TestLogMailer(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, NewLogMailer(slog.New(slog.NewJSONHandler(&buf, nil)), true).Send(t.Context(), "a@example.com", "Hi", "<p>link</p>"))
	var rec map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rec))
	assert.Equal(t, "email (log mailer)", rec["msg"])
	assert.Equal(t, "a@example.com", rec["to"])
	assert.Equal(t, "Hi", rec["subject"])
	assert.Equal(t, "<p>link</p>", rec["body"])
}

func TestLogMailerProductionOmitsBody(t *testing.T) {
	var buf bytes.Buffer
	token := "tok_" + strings.Repeat("b", 39)
	require.NoError(t, NewLogMailer(slog.New(slog.NewJSONHandler(&buf, nil)), false).Send(t.Context(), "a@example.com", "Hi", "https://w.example/"+token))
	var rec map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rec))
	assert.NotContains(t, rec, "body")
	assert.NotContains(t, buf.String(), token)
}

func TestSMTPMessage(t *testing.T) {
	msg, err := newMessage("noreply@example.com", "a@example.com", "Hi", "<p>x</p>")
	require.NoError(t, err)
	var buf bytes.Buffer
	_, err = msg.WriteTo(&buf)
	require.NoError(t, err)
	for _, s := range []string{"Subject: Hi", "a@example.com", "text/html"} {
		assert.Contains(t, buf.String(), s)
	}
}
