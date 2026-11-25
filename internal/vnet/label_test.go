package vnet

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOrgLabel(t *testing.T) {
	id := uuid.MustParse("0a1b2c3d-1111-2222-3333-444455556666")
	require.Equal(t, "acme-corp", OrgLabel("Acme Corp", id))
	require.Equal(t, "0a1b2c3d", OrgLabel("--Ü--", id))
	require.Equal(t, strings.Repeat("a", 63), OrgLabel(strings.Repeat("a", 80), id))
	require.Equal(t, "a", OrgLabel(strings.Repeat("a", 1)+" ", id))
}
