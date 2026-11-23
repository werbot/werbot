// Package rpcid converts ids between their API text and UUIDs.
package rpcid

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// One parses s; a malformed id is an apperr.ErrInvalidArgument naming it.
func One(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: invalid id %q", apperr.ErrInvalidArgument, s)
	}
	return id, nil
}

// Optional parses s as One does; empty is nil.
func Optional(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := One(s)
	return &id, err
}

// String is the text of id; nil is empty.
func String(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

// Parse parses ids as One does. No ids give nil.
func Parse(ss []string) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, s := range ss {
		id, err := One(s)
		if err != nil {
			return nil, err
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
