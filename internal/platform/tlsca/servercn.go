package tlsca

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// ServerCN is the client certificate common name of one server replica.
func ServerCN(instanceID uuid.UUID) string { return "server:" + instanceID.String() }

// ParseServerCN reverses ServerCN; only the canonical form is accepted.
func ParseServerCN(cn string) (uuid.UUID, error) {
	rest, ok := strings.CutPrefix(cn, "server:")
	if !ok {
		return uuid.Nil, fmt.Errorf("%w: bad server common name", apperr.ErrInvalidArgument)
	}
	id, err := uuid.Parse(rest)
	if err != nil || ServerCN(id) != cn {
		return uuid.Nil, fmt.Errorf("%w: non-canonical server common name", apperr.ErrInvalidArgument)
	}
	return id, nil
}
