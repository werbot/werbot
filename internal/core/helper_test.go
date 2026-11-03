package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func Test_ensureValidToken(t *testing.T) {
	okHandler := func(_ context.Context, req any) (any, error) {
		return req, nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	testCases := []struct {
		name      string
		tokenEnv  string
		ctx       context.Context
		wantCode  codes.Code
		wantCall  bool
		wantError string
	}{
		{
			name:     "missing metadata",
			tokenEnv: "secret",
			ctx:      context.Background(),
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "missing authorization header",
			tokenEnv: "secret",
			ctx:      metadata.NewIncomingContext(context.Background(), metadata.Pairs("other", "value")),
			wantCode: codes.Unauthenticated,
		},
		{
			name:      "token is not configured",
			tokenEnv:  "",
			ctx:       metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer anything")),
			wantCode:  codes.Unauthenticated,
			wantError: "Server authorization token is not configured",
		},
		{
			name:     "wrong token",
			tokenEnv: "secret",
			ctx:      metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token")),
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "valid token",
			tokenEnv: "secret",
			ctx:      metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer secret")),
			wantCall: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GRPCSERVER_TOKEN", tt.tokenEnv)

			called := false
			handler := func(ctx context.Context, req any) (any, error) {
				called = true
				return okHandler(ctx, req)
			}

			_, err := ensureValidToken(tt.ctx, "request", info, handler)

			assert.Equal(t, tt.wantCall, called)
			if tt.wantCall {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Equal(t, tt.wantCode, status.Code(err))
			if tt.wantError != "" {
				assert.Equal(t, tt.wantError, status.Convert(err).Message())
			}
		})
	}
}

func Test_unaryRecoveryInterceptor(t *testing.T) {
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Panic"}
	panicHandler := func(_ context.Context, _ any) (any, error) {
		panic("boom")
	}

	response, err := unaryRecoveryInterceptor(context.Background(), "request", info, panicHandler)

	assert.Nil(t, response)
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func Test_unaryRecoveryInterceptor_passesThrough(t *testing.T) {
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Ok"}
	okHandler := func(_ context.Context, req any) (any, error) {
		return req, nil
	}

	response, err := unaryRecoveryInterceptor(context.Background(), "request", info, okHandler)

	require.NoError(t, err)
	assert.Equal(t, "request", response)
}
