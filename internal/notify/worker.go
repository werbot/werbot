package notify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/riverqueue/river"

	"github.com/werbot/werbot/internal/platform/secretbox"
)

// Worker sends the emails of SendEmailArgs jobs.
type Worker struct {
	river.WorkerDefaults[SendEmailArgs]
	mailer Mailer
	box    *secretbox.Box
}

// NewWorker returns a Worker that opens job data with box and sends via m.
func NewWorker(m Mailer, box *secretbox.Box) *Worker { return &Worker{mailer: m, box: box} }

// Work opens and renders the email, then sends it. An unopenable or
// unrenderable job can never succeed, so it is cancelled instead of retried.
func (w *Worker) Work(ctx context.Context, job *river.Job[SendEmailArgs]) error {
	a := job.Args
	plain, err := w.box.Open(a.SealedData, a.KeyID)
	if err != nil {
		return river.JobCancel(err)
	}
	var data map[string]string
	if err := json.Unmarshal(plain, &data); err != nil {
		return river.JobCancel(fmt.Errorf("notify: decode data: %w", err))
	}
	subject, body, err := Render(a.Template, data)
	if err != nil {
		return river.JobCancel(err)
	}
	return w.mailer.Send(ctx, a.To, subject, body)
}
