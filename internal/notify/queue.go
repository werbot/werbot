package notify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/werbot/werbot/internal/platform/secretbox"
)

// Queue inserts email jobs inside the caller's transaction.
type Queue struct {
	client *river.Client[pgx.Tx]
	box    *secretbox.Box
}

// NewQueue returns a Queue that seals job data with box.
func NewQueue(client *river.Client[pgx.Tx], box *secretbox.Box) *Queue {
	return &Queue{client: client, box: box}
}

// EnqueueTx seals data and inserts a send_email job in tx, so the email is
// queued only if the transaction commits.
func (q *Queue) EnqueueTx(ctx context.Context, tx pgx.Tx, to, template string, data map[string]string) error {
	if _, ok := templates[template]; !ok {
		return fmt.Errorf("notify: unknown template %q", template)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("notify: encode data: %w", err)
	}
	sealed, keyID, err := q.box.Seal(raw)
	if err != nil {
		return err
	}
	_, err = q.client.InsertTx(ctx, tx, SendEmailArgs{To: to, Template: template, SealedData: sealed, KeyID: keyID}, nil)
	return err
}
