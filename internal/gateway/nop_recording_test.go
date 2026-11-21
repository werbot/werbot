package gateway

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNopRecordingMarker(t *testing.T) {
	assert.NotPanics(t, func() { NopRecording(uuid.New(), 80, 24).Marker("x") })
}
