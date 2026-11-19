package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

func TestLs(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, h.run("ls"))
	assert.Regexp(t, `NAME\s+TYPE\s+KINDS\s+GATEWAY\ndb\s+ssh\s+terminal,exec,stream\s+gw\.example\.com:2222\n`, h.out.String())

	h.out.Reset()
	h.resources = []*werbotv1.AccessibleResource{}
	require.NoError(t, h.run("ls"))
	assert.Equal(t, "No accessible resources\n", h.out.String())
}
