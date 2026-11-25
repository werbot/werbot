package mcpserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// harness runs Run over two pipes; stdin closes on stop or cleanup. One
// goroutine reads stdout into lines.
type harness struct {
	stdin *io.PipeWriter
	lines chan []byte
	done  chan error
	once  sync.Once
	err   error
}

func startRun(t *testing.T, opts Options) *harness {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	h := &harness{stdin: inW, lines: make(chan []byte, 64), done: make(chan error, 1)}
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(nil, 1<<20)
		for sc.Scan() {
			h.lines <- bytes.Clone(sc.Bytes())
		}
		close(h.lines)
	}()
	go func() {
		err := Run(context.Background(), inR, outW, opts)
		_ = outW.Close()
		h.done <- err
	}()
	t.Cleanup(func() { _ = h.stop(t) })
	return h
}

// stop closes stdin (EOF), drains stdout and returns what Run returned.
func (h *harness) stop(t *testing.T) error {
	t.Helper()
	h.once.Do(func() {
		_ = h.stdin.Close()
		go func() {
			for range h.lines { // drain stdout so Run can finish
			}
		}()
		select {
		case h.err = <-h.done:
		case <-time.After(10 * time.Second):
			t.Error("Run did not return after stdin EOF")
		}
	})
	return h.err
}

// sdk connects the go-sdk client named "claude-code" (protocol 2026-07-28: no initialize).
func (h *harness) sdk(t *testing.T) *mcp.ClientSession {
	t.Helper()
	pr, pw := io.Pipe()
	go func() {
		for line := range h.lines {
			_, _ = pw.Write(append(line, '\n'))
		}
		_ = pw.Close()
	}()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "claude-code", Version: "1.0"}, nil).
		Connect(t.Context(), &mcp.IOTransport{Reader: pr, Writer: nopWriteCloser{h.stdin}}, nil)
	require.NoError(t, err)
	return cs
}

func callTool(t *testing.T, cs *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	return res
}

func errText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	require.True(t, res.IsError, "the call must fail")
	require.NotEmpty(t, res.Content)
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	return tc.Text
}

func decode[T any](t *testing.T, res *mcp.CallToolResult) T {
	t.Helper()
	require.False(t, res.IsError, "unexpected tool error: %v", res.Content)
	raw, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var v T
	require.NoError(t, json.Unmarshal(raw, &v))
	return v
}

// raw sends one JSON-RPC message; with id > 0 it reads lines up to the answer.
func (h *harness) raw(t *testing.T, id int, method string, params any) map[string]any {
	t.Helper()
	msg := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id > 0 {
		msg["id"] = id
	}
	line, err := json.Marshal(msg)
	require.NoError(t, err)
	_, err = h.stdin.Write(append(line, '\n'))
	require.NoError(t, err)
	if id == 0 {
		return nil
	}
	for line := range h.lines {
		got := jsonRPC(t, line)
		if got["id"] == float64(id) {
			return got
		}
	}
	return nil
}

// jsonRPC decodes one stdout line, which must be a JSON-RPC 2.0 message.
func jsonRPC(t *testing.T, line []byte) map[string]any {
	t.Helper()
	var msg map[string]any
	require.NoError(t, json.Unmarshal(line, &msg), "stdout line is not JSON: %q", line)
	require.Equal(t, "2.0", msg["jsonrpc"], "stdout line is not JSON-RPC 2.0: %q", line)
	return msg
}

func (h *harness) initialize(t *testing.T) {
	t.Helper()
	res := h.raw(t, 1, "initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "claude-code", "version": "1.0"}})
	require.NotNil(t, res["result"], "initialize: %v", res)
	h.raw(t, 0, "notifications/initialized", map[string]any{})
}

// lockedWriter serializes writes of concurrent log calls.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
