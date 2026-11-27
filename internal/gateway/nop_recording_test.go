package gateway

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNopRecordingMarker(t *testing.T) {
	assert.NotPanics(t, func() { NopRecording(uuid.New(), 80, 24, "").Marker("x") })
}

func TestNopRecordingEvent(t *testing.T) {
	assert.NoError(t, NopRecording(uuid.New(), 0, 0, "querylog").Event(map[string]string{"kind": "query"}))
}
