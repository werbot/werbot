package gateway

import (
	"bytes"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"
)

func TestCopyBuffer(t *testing.T) {
	in := strings.Repeat("x", 100<<10)
	var out bytes.Buffer
	n, err := CopyBuffer(&out, iotest.OneByteReader(strings.NewReader(in[:50])))
	require.NoError(t, err)
	require.EqualValues(t, 50, n)
	out.Reset()
	n, err = CopyBuffer(&out, strings.NewReader(in))
	require.NoError(t, err)
	require.EqualValues(t, len(in), n)
	require.Equal(t, in, out.String())
}
