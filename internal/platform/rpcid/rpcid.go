// Package rpcid converts lists of ids between their API text and UUIDs.
package rpcid

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// Parse parses ids; a malformed one is an apperr.ErrInvalidArgument naming it.
// No ids give nil.
func Parse(ss []string) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, s := range ss {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid id %q", apperr.ErrInvalidArgument, s)
		}
		out = append(out, id)
	}
	return out, nil
}

// Strings is the text of ids; no ids give nil.
func Strings(ids []uuid.UUID) (out []string) {
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}
