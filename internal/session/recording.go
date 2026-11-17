package session

import (
	"context"
	"io"

	"github.com/google/uuid"
)

// RecordingStore persists gzip-compressed recording chunks.
type RecordingStore interface {
	// Put stores chunk seq; a repeated Put of the same seq overwrites it.
	Put(ctx context.Context, orgID, sessionID uuid.UUID, seq int, gz []byte) error
	// Get opens chunk seq; a missing object returns an error wrapping fs.ErrNotExist.
	Get(ctx context.Context, orgID, sessionID uuid.UUID, seq int) (io.ReadCloser, error)
}
