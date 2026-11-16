// Package apperr defines domain error sentinels and maps them to Connect errors.
package apperr

import (
	"context"
	"errors"

	"connectrpc.com/connect"
)

// Sentinels: wrap with %w; ToConnect maps each to its Connect code.
var (
	ErrNotFound           = errors.New("not found")
	ErrAlreadyExists      = errors.New("already exists")
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrFailedPrecondition = errors.New("failed precondition")
	ErrUnavailable        = errors.New("unavailable") // temporary (database, record store): the client retries
)

var codes = []struct {
	err  error
	code connect.Code
}{
	{ErrNotFound, connect.CodeNotFound},
	{ErrAlreadyExists, connect.CodeAlreadyExists},
	{ErrInvalidArgument, connect.CodeInvalidArgument},
	{ErrPermissionDenied, connect.CodePermissionDenied},
	{ErrUnauthenticated, connect.CodeUnauthenticated},
	{ErrFailedPrecondition, connect.CodeFailedPrecondition},
	{ErrUnavailable, connect.CodeUnavailable},
}

// ToConnect converts err to a *connect.Error. Unknown causes never reach the
// client: the message is masked and the cause stays reachable through Unwrap.
func ToConnect(err error) error {
	if err == nil {
		return nil
	}
	if ce := (*connect.Error)(nil); errors.As(err, &ce) {
		return err
	}
	for _, c := range codes {
		if errors.Is(err, c.err) {
			return connect.NewError(c.code, err)
		}
	}
	switch {
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, &maskedError{"canceled", err})
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeUnavailable, &maskedError{"unavailable", err})
	}
	return connect.NewError(connect.CodeInternal, &maskedError{"internal error", err})
}

type maskedError struct {
	msg   string
	cause error
}

func (e *maskedError) Error() string { return e.msg }
func (e *maskedError) Unwrap() error { return e.cause }
