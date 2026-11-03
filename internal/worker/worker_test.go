package worker

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func Test_New_returnsErrorOnInvalidRedisURI(t *testing.T) {
	server, err := New("not-a-redis-uri", nil)

	assert.Nil(t, server)
	require.Error(t, err)
}

func Test_cronUpdateFirewallList_returnsError(t *testing.T) {
	conn, err := grpc.NewClient(
		"passthrough:///unreachable",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return nil, errors.New("dial failed")
		}),
	)
	require.NoError(t, err)
	defer conn.Close()

	err = cronUpdateFirewallList(conn)(context.Background())

	require.Error(t, err)
}
