package cli

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sharedWarningStart = "warning: the agent runs as your OS user and can read your werbot profile in "

func TestMCPCommandStdio(t *testing.T) {
	h := newHarness(t)
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	h.env.Stdin, h.env.Stdout = inR, outW
	done := make(chan error, 1)
	go func() { done <- h.run("mcp"); _ = outW.Close() }()

	_, err := io.WriteString(inW, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`+"\n")
	require.NoError(t, err)
	line, err := bufio.NewReader(outR).ReadBytes('\n')
	require.NoError(t, err)
	var msg struct {
		Result struct {
			ServerInfo struct{ Name string } `json:"serverInfo"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(line, &msg), "stdout carries JSON-RPC")
	assert.Equal(t, "werbot", msg.Result.ServerInfo.Name)

	require.NoError(t, inW.Close())
	go func() { _, _ = io.Copy(io.Discard, outR) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("mcp did not stop at stdin EOF")
	}
	assert.NotContains(t, h.errOut.String(), `"jsonrpc"`, "stderr carries no JSON-RPC")
}

// Shared mode: one warning line on stderr, none on stdout.
func TestSharedModeWarnsOnce(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, h.run("mcp"))
	assert.Equal(t, 1, strings.Count(h.errOut.String(), sharedWarningStart))
	assert.Contains(t, h.errOut.String(), sharedWarningStart+h.home+`; isolate it with "werbot agents profile create"`+"\n")
	assert.Empty(t, h.out.String())
}

// A refused login is the plain not-logged-in error, not an MCP exit code.
func TestMCPNotLoggedIn(t *testing.T) {
	h := newHarness(t)
	h.logout(t)
	assert.ErrorContains(t, h.run("mcp"), "not logged in")
}
