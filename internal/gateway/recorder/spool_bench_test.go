package recorder_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/gateway/recorder"
)

// BenchmarkSpoolSessionEnd is the spool work of Pipeline.finish for a short
// recorded session: the end, the last (here: only) chunk, then the close.
func BenchmarkSpoolSessionEnd(b *testing.B) {
	sp, err := recorder.OpenSpool(b.TempDir(), &fakeUploader{}, recorder.SpoolOptions{}, slog.New(slog.DiscardHandler))
	if err != nil {
		b.Fatal(err)
	}
	hash, gz := bytes.Repeat([]byte{1}, 32), make([]byte, 200)
	for b.Loop() {
		id := uuid.New()
		if err := sp.End(id, "closed", nil); err != nil {
			b.Fatal(err)
		}
		if err := sp.Append(id, 0, hash, gz); err != nil {
			b.Fatal(err)
		}
		if err := sp.Close(id); err != nil {
			b.Fatal(err)
		}
	}
}
