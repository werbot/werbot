package rpcid

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
)

func TestParseAndStrings(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	ids, err := Parse([]string{a.String(), b.String()})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{a, b}, ids)
	assert.Equal(t, []string{a.String(), b.String()}, Strings(ids))

	ids, err = Parse(nil)
	require.NoError(t, err)
	assert.Nil(t, ids)
	assert.Nil(t, Strings(nil))

	_, err = Parse([]string{a.String(), "nope"})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)
	assert.Contains(t, err.Error(), `"nope"`)
}
