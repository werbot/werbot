package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewClient_failsWithoutToken(t *testing.T) {
	t.Setenv("GRPCSERVER_TOKEN", "")

	conn, err := NewClient()

	assert.Nil(t, conn)
	require.Error(t, err)
	assert.Equal(t, "GRPCSERVER_TOKEN is not set", err.Error())
}
