package apperr_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
)

func TestToConnect(t *testing.T) {
	assert.NoError(t, apperr.ToConnect(nil))
	for err, code := range map[error]connect.Code{
		apperr.ErrNotFound:           connect.CodeNotFound,
		apperr.ErrAlreadyExists:      connect.CodeAlreadyExists,
		apperr.ErrInvalidArgument:    connect.CodeInvalidArgument,
		apperr.ErrPermissionDenied:   connect.CodePermissionDenied,
		apperr.ErrUnauthenticated:    connect.CodeUnauthenticated,
		apperr.ErrFailedPrecondition: connect.CodeFailedPrecondition,
		apperr.ErrUnavailable:        connect.CodeUnavailable,
	} {
		assert.Equal(t, code, connect.CodeOf(apperr.ToConnect(fmt.Errorf("x: %w", err))), err.Error())
	}

	existing := connect.NewError(connect.CodeAborted, errors.New("a"))
	assert.Same(t, existing, apperr.ToConnect(existing))

	cause := errors.New("pq: password=secret")
	var ce *connect.Error
	require.ErrorAs(t, apperr.ToConnect(cause), &ce)
	assert.Equal(t, connect.CodeInternal, ce.Code())
	assert.Equal(t, "internal error", ce.Message())
	assert.ErrorIs(t, ce, cause)

	dl := fmt.Errorf("failed to connect to `user=postgres`: %w", context.DeadlineExceeded)
	require.ErrorAs(t, apperr.ToConnect(dl), &ce)
	assert.Equal(t, connect.CodeUnavailable, ce.Code())
	assert.Equal(t, "unavailable", ce.Message())
	assert.ErrorIs(t, ce, context.DeadlineExceeded)

	cn := fmt.Errorf("query: %w", context.Canceled)
	require.ErrorAs(t, apperr.ToConnect(cn), &ce)
	assert.Equal(t, connect.CodeCanceled, ce.Code())
	assert.Equal(t, "canceled", ce.Message())
	assert.ErrorIs(t, ce, context.Canceled)
}
