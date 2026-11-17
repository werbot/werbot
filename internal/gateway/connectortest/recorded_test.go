package connectortest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
)

// failT records a failed require instead of ending the test.
type failT struct{ msg string }

func (f *failT) Errorf(format string, args ...any) { f.msg = fmt.Sprintf(format, args...) }
func (*failT) FailNow()                            { panic("failed") }

func failed(fn func(t *failT)) (f *failT) {
	f = &failT{}
	defer func() { _ = recover() }()
	fn(f)
	return f
}

// A connector that records a kind its type does not record fails Run.
func TestRunRequiresRecordingOnlyForRecorded(t *testing.T) {
	rec := &spySink{}
	rec.Output(nil)
	f := failed(func(f *failT) { requireRecorded(f, "echo", gateway.KindStream, rec) })
	require.Contains(t, f.msg, "which its type does not record")

	require.Empty(t, failed(func(f *failT) { requireRecorded(f, "echo", gateway.KindStream, &spySink{}) }).msg)
}

type marked struct{ Echo }

func (marked) DialsTarget() {}

func TestRunDialsThroughSession(t *testing.T) {
	require.Contains(t, failed(func(f *failT) { requireDialsThroughSession(f, marked{}, 0) }).msg, "must dial")
	require.Contains(t, failed(func(f *failT) { requireDialsThroughSession(f, Echo{}, 1) }).msg, "lacks DialsTarget")
	require.Empty(t, failed(func(f *failT) { requireDialsThroughSession(f, marked{}, 1) }).msg)
	require.Empty(t, failed(func(f *failT) { requireDialsThroughSession(f, Echo{}, 0) }).msg)
}
