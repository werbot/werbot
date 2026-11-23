// Package mcpserver is werbot mcp: an MCP server on stdio that acts as one
// agent run of the logged-in sponsor. The run starts on the first tools/call
// and ends when stdin closes. stdout carries only JSON-RPC; logs go to slog.
package mcpserver

import (
	"cmp"
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/connectcli"
	"github.com/werbot/werbot/internal/platform/version"
)

// Options configure Run. Profile is the sponsor's profile or an agent
// profile (Profile.Agent set); Agent names the agent, else the client does.
type Options struct {
	Profile       *clientauth.Profile
	Agent, Reason string
	Dial          connectcli.Dialer // nil: connectcli.Dial
}

// endTimeout bounds EndRun after stdin closed.
const endTimeout = 10 * time.Second

// server is one werbot mcp process: at most one agent run.
type server struct {
	opts Options
	mcp  *mcp.Server

	mu  sync.Mutex // one exchange at a time
	run *clientauth.AgentProfile

	cache resourceCache
}

// Run serves MCP on in/out until in reaches EOF or ctx ends, then ends the
// run if one started. A failed exchange is a tool error, never an exit.
func Run(ctx context.Context, in io.Reader, out io.Writer, opts Options) error {
	if opts.Dial == nil {
		opts.Dial = connectcli.Dial
	}
	s := &server{opts: opts, mcp: mcp.NewServer(&mcp.Implementation{Name: "werbot", Version: version.Version}, nil)}
	s.addTools()
	err := s.mcp.Run(ctx, &mcp.IOTransport{Reader: io.NopCloser(in), Writer: nopWriteCloser{out}})
	s.end(ctx)
	return err
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// agent returns the run, starting it on the first call: the client name
// comes from the request (_meta or initialize), else "unknown".
func (s *server) agent(ctx context.Context, req *mcp.CallToolRequest) (*clientauth.AgentProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.run != nil {
		return s.run, nil
	}
	client := "unknown"
	if ci := req.ClientInfo(); ci != nil && ci.Name != "" {
		client = ci.Name
	}
	run, err := s.opts.Profile.ExchangeAgent(ctx, s.agentName(client), client, s.opts.Reason)
	if err != nil {
		return nil, err
	}
	s.run = run
	return run, nil
}

// agentName is the agent of the run: an agent profile's own (ExchangeAgent
// refuses another flag), else the flag or the client name.
func (s *server) agentName(client string) string {
	return AgentName(cmp.Or(s.opts.Agent, s.opts.Profile.Agent), client)
}

// end finishes the run (client_exit) even after ctx ended.
func (s *server) end(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.run == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), endTimeout)
	defer cancel()
	if err := s.run.End(ctx); err != nil {
		slog.Warn("mcp: end the agent run", "run_id", s.run.RunID, "error", err)
	}
}

// AgentName is flag when set; else client in lower case with every byte
// outside [a-z0-9_-] replaced by "-", cut to 63; empty: "agent".
func AgentName(flag, client string) string {
	if flag != "" {
		return flag
	}
	name := strings.Map(func(r rune) rune {
		if r == '_' || r == '-' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' {
			return r
		}
		return '-'
	}, strings.ToLower(client))
	return cmp.Or(name[:min(len(name), 63)], "agent")
}
