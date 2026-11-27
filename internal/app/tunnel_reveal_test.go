package app

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A type that accepts no credential can have none stored: Reveal answers the
// empty secret without the credential service (nil here).
func TestRevealSkipsTypesWithoutCredentials(t *testing.T) {
	for _, typ := range []string{"tcp", "udp"} {
		sec, err := tunnelCredentials{}.Reveal(context.Background(), uuid.New(), uuid.New(), typ, false)
		require.NoError(t, err, typ)
		assert.Empty(t, sec, typ)
	}
}
