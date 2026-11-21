package recorder_test

import (
	"context"
	"strings"
	"testing"
	"testing/synctest"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway/recorder"
)

func TestRecorderMarker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sink := &memSink{}
		r := recorder.New(uuid.Must(uuid.NewV7()), 80, 24, sink, recorder.Options{})
		b := []byte("б")
		r.Output(append([]byte("a"), b[0])) // rune split around the marker
		time.Sleep(50 * time.Millisecond)
		r.Marker(`ls "x" \ y [read]`)
		r.Output(b[1:])
		require.NoError(t, r.Close(context.Background()))

		events := assertCastParses(t, sink.cast(t))
		assert.Equal(t, []event{{"o", "a"}, {"m", `ls "x" \ y [read]`}, {"o", "б"}}, events)
		assert.Contains(t, string(sink.cast(t)), `,"m","ls \"x\" \\ y [read]"]`)
		assertChain(t, sink)
	})
}

func TestRecorderMarkerTruncated(t *testing.T) {
	sink := &memSink{}
	r := recorder.New(uuid.Must(uuid.NewV7()), 80, 24, sink, recorder.Options{})
	r.Marker(strings.Repeat("я", 1000)) // 2000 bytes
	require.NoError(t, r.Close(context.Background()))

	events := assertCastParses(t, sink.cast(t))
	require.Len(t, events, 1)
	assert.Equal(t, "m", events[0].code)
	assert.LessOrEqual(t, len(events[0].data), recorder.MaxMarkerBytes)
	assert.True(t, utf8.ValidString(events[0].data))
	assert.Len(t, events[0].data, recorder.MaxMarkerBytes)
	assertChain(t, sink)
}
