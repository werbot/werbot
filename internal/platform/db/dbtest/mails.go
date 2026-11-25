package dbtest

import (
	"context"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Mail is one email a Mails recorder queued.
type Mail struct {
	To, Template string
	Data         map[string]string
}

// Mails records EnqueueTx calls instead of inserting River jobs.
type Mails struct {
	mu   sync.Mutex
	sent []Mail
}

// EnqueueTx records the email.
func (q *Mails) EnqueueTx(_ context.Context, _ pgx.Tx, to, template string, data map[string]string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.sent = append(q.sent, Mail{to, template, data})
	return nil
}

// Sent returns every recorded email in order.
func (q *Mails) Sent() []Mail {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]Mail(nil), q.sent...)
}

// Of returns the emails queued with template.
func (q *Mails) Of(template string) []Mail {
	var out []Mail
	for _, m := range q.Sent() {
		if m.Template == template {
			out = append(out, m)
		}
	}
	return out
}

// To returns the recipients of template.
func (q *Mails) To(template string) []string {
	var out []string
	for _, m := range q.Of(template) {
		out = append(out, m.To)
	}
	return out
}

// Reset forgets the recorded emails.
func (q *Mails) Reset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.sent = nil
}

// AuditActions lists the audit actions of the database in insertion order (ids are uuidv7).
func AuditActions(t testing.TB, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), "SELECT action FROM audit_event ORDER BY id")
	if err != nil {
		t.Fatalf("dbtest: audit actions: %v", err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("dbtest: audit actions: %v", err)
	}
	return out
}
