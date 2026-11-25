//go:build !linux && !darwin

package vnet

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunUnsupported(t *testing.T) {
	require.EqualError(t, Run(t.Context(), Options{}), "--vnet is not supported on "+runtime.GOOS)
}
