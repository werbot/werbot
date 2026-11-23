package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/connectcli"
	"github.com/werbot/werbot/internal/mcpserver"
	"github.com/werbot/werbot/internal/platform/sshca"
)

const (
	runClient = "werbot-agent-run"
	// renewEvery checks the run certificate twice per renew window.
	renewEvery = sshca.RenewBefore / 2
	// killAfter is how long a child may take to leave after the interrupt.
	killAfter = 2 * time.Second
	// endTimeout bounds EndRun after the child left.
	endTimeout = 10 * time.Second
)

func newAgentCommand(env Env) *cobra.Command {
	cmd := &cobra.Command{Use: "agent", Short: "Run a command as an agent of your account"}
	cmd.AddCommand(newAgentRunCommand(env))
	return cmd
}

type runOptions struct {
	agent, reason string
	resources     []string
}

func newAgentRunCommand(env Env) *cobra.Command {
	var o runOptions
	cmd := &cobra.Command{
		Use:   "run [--agent <name>] [--reason <text>] [--resource <name>]... -- <cmd> [args]",
		Short: "Run a command with ssh set up for an agent run", Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return runAgent(cmd.Context(), env, o, args) },
	}
	cmd.Flags().SetInterspersed(false) // flags after the command belong to it
	cmd.Flags().StringVar(&o.agent, "agent", "", "agent name (default: the command name)")
	cmd.Flags().StringVar(&o.reason, "reason", "", "why the agent runs, shown to approvers")
	cmd.Flags().StringSliceVar(&o.resources, "resource", nil, "resource to set up (default: every ssh resource); stream and datagram resources open a local port")
	return cmd
}

// runAgent starts one agent run, runs argv with the run directory on PATH and
// ends the run when argv leaves; the run directory is removed in every case.
func runAgent(ctx context.Context, env Env, o runOptions, argv []string) error {
	home, p, err := loadAnyProfile(env)
	if err != nil {
		return err
	}
	agent, err := agentMode(env, home, p, o.agent)
	if err != nil {
		return err
	}
	if agent == "" {
		agent = mcpserver.AgentName("", filepath.Base(argv[0]))
	}
	run, err := p.ExchangeAgent(ctx, agent, runClient, o.reason)
	if err != nil {
		return err
	}
	dir := filepath.Join(home, "runs", run.RunID.String())
	defer finishRun(ctx, env, run, dir)
	return serveRun(ctx, env, run, dir, o.resources, argv)
}

// serveRun sets the run directory up, keeps its certificate fresh and local
// ports open while argv runs, and returns what argv returned.
func serveRun(ctx context.Context, env Env, run *clientauth.AgentProfile, dir string, names, argv []string) error {
	res, err := werbotv1connect.NewResourceServiceClient(run.HTTPClient(), run.Server).
		ListAccessibleResources(ctx, connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	if err != nil {
		return err
	}
	sel, specs, err := selectResources(res.Msg.GetResources(), names)
	if err != nil {
		return err
	}
	if _, err := run.Signer(ctx); err != nil {
		return err
	}
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		sshPath = "ssh" // the wrapper then reports the missing binary when it is used
	}
	if err := writeRunDir(dir, run, sel, sshPath, runtime.GOOS); err != nil {
		return err
	}
	cfg := connectcli.Config{Login: run.Login, KnownHosts: filepath.Join(dir, "known_hosts"), Specs: specs, Signer: run.Signer}
	if len(specs) > 0 {
		if cfg.Gateway, err = sharedGateway(sel, specs); err != nil {
			return err
		}
	}
	printBanner(env, run, wrapperPath(dir, runtime.GOOS))

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer func() { cancel(); wg.Wait() }()
	wg.Go(func() { keepFresh(ctx, run, dir, renewEvery, env.Stderr) })
	if len(specs) > 0 {
		wg.Go(func() {
			if err := runConnect(ctx, cfg, env.Stderr, env.Stderr); err != nil {
				_, _ = fmt.Fprintf(env.Stderr, "werbot connect: %v\n", err)
			}
		})
	}
	return runChild(ctx, env, filepath.Join(dir, "bin"), argv)
}

// selectResources returns the resources to set up and a Spec for each one
// that is not ssh; no names means every ssh resource.
func selectResources(rs []*werbotv1.AccessibleResource, names []string) (sel []*werbotv1.AccessibleResource, specs []connectcli.Spec, err error) {
	if len(names) == 0 {
		for _, r := range rs {
			if r.GetType() == "ssh" {
				sel = append(sel, r)
			}
		}
		return sel, nil, nil
	}
	for _, name := range names {
		r, err := find(rs, name)
		if err != nil {
			return nil, nil, err
		}
		sel = append(sel, r)
		if r.GetType() != "ssh" {
			specs = append(specs, connectcli.Spec{Resource: name})
		}
	}
	return sel, specs, nil
}

func printBanner(env Env, run *clientauth.AgentProfile, wrapper string) {
	_, _ = fmt.Fprintf(env.Stderr, "Agent run %s (agent %s); ssh: %s\nMCP: {\"mcpServers\":{\"werbot\":{\"command\":\"werbot\",\"args\":[\"mcp\",\"--agent\",%q]}}}\n",
		run.RunID, run.Agent, wrapper, run.Agent)
}

// runChild runs argv with bin first on PATH and without the profile location,
// and forwards the interrupt of ctx; it kills the child killAfter later.
func runChild(ctx context.Context, env Env, bin string, argv []string) error {
	c := exec.CommandContext(ctx, resolveCommand(argv[0], bin, runtime.GOOS), argv[1:]...) //nolint:gosec // the command is the user's own
	c.Env = childEnv(os.Environ(), bin)
	c.Stdin, c.Stdout, c.Stderr = env.Stdin, env.Stdout, env.Stderr
	c.Cancel = func() error {
		if err := c.Process.Signal(os.Interrupt); err != nil { // Windows has no interrupt
			return c.Process.Kill()
		}
		return nil
	}
	c.WaitDelay = killAfter
	return c.Run() // an *exec.ExitError carries the child's exit code to the process
}

// resolveCommand looks a bare name up in bin before the parent's PATH, so
// "ssh" is the run wrapper and never the system ssh.
func resolveCommand(name, bin, goos string) string {
	if strings.ContainsAny(name, `/\`) {
		return name
	}
	exts := []string{""}
	if goos == "windows" {
		exts = append(exts, ".cmd")
	}
	for _, ext := range exts {
		p := filepath.Join(bin, name+ext)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return name
}

// childEnv is environ with bin first on PATH, an empty SSH_AUTH_SOCK (the
// sponsor's agent stays hidden) and no WERBOT_HOME or WERBOT_SERVER.
func childEnv(environ []string, bin string) []string {
	out := make([]string, 0, len(environ)+1)
	path := ""
	for _, kv := range environ {
		switch k, v, _ := strings.Cut(kv, "="); {
		case k == "WERBOT_HOME" || k == "WERBOT_SERVER" || k == "SSH_AUTH_SOCK":
		case strings.EqualFold(k, "PATH"):
			path = v
		default:
			out = append(out, kv)
		}
	}
	return append(out, "SSH_AUTH_SOCK=", "PATH="+bin+string(os.PathListSeparator)+path)
}

// keepFresh renews the run certificate in dir every interval until ctx ends.
func keepFresh(ctx context.Context, run *clientauth.AgentProfile, dir string, every time.Duration, stderr io.Writer) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		_, err := run.Signer(ctx)
		if err == nil {
			err = run.WriteKey(dir)
		}
		if err != nil && ctx.Err() == nil {
			_, _ = fmt.Fprintf(stderr, "werbot: renew the run certificate: %v\n", err)
		}
	}
}

// finishRun ends the run on the server, even after ctx ended, and removes dir.
func finishRun(ctx context.Context, env Env, run *clientauth.AgentProfile, dir string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), endTimeout)
	defer cancel()
	if err := run.End(ctx); err != nil {
		_, _ = fmt.Fprintf(env.Stderr, "werbot: end the agent run: %v\n", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		_, _ = fmt.Fprintf(env.Stderr, "werbot: remove %s: %v\n", dir, err)
	}
}
