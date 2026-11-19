package app

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/werbot/werbot/internal/cli"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db"
	"github.com/werbot/werbot/internal/platform/logging"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/platform/version"
	"github.com/werbot/werbot/internal/webui"
	"github.com/werbot/werbot/sdk"
	"github.com/werbot/werbot/web"
)

// Run executes the command in args and returns when it is done or ctx ends.
// A -help flag is not an error.
func Run(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error {
	env := cli.Env{Getenv: getenv, Stdin: os.Stdin, Stdout: stdout, Stderr: os.Stderr, OpenBrowser: cli.OpenURL}
	root := NewRootCommand(env)
	if args == nil {
		args = []string{}
	}
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); !errors.Is(err, flag.ErrHelp) {
		return err
	}
	return nil
}

// NewRootCommand builds the werbot command: the server roles and the client commands.
// server, gateway and connect parse their own flags (DisableFlagParsing), so
// their single-dash flags and --help work as before.
func NewRootCommand(env cli.Env) *cobra.Command {
	root := &cobra.Command{Use: "werbot", SilenceUsage: true, SilenceErrors: true}
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)
	root.AddCommand(
		&cobra.Command{
			Use: "server", Short: "Run the API server (--with-gateway adds the \"local\" gateway pool)", DisableFlagParsing: true,
			RunE: func(cmd *cobra.Command, args []string) error { return runServer(cmd.Context(), args, env) },
		},
		&cobra.Command{
			Use: "gateway", Short: "Run an SSH gateway", DisableFlagParsing: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				gcfg, err := gateway.LoadConfig(args, env.Getenv)
				if err != nil {
					return err
				}
				level, format, err := config.LoadLogging(env.Getenv)
				if err != nil {
					return err
				}
				return gateway.Run(cmd.Context(), gcfg, logging.New(env.Stdout, level, format))
			},
		},
		&cobra.Command{
			Use: "web", Short: "Run the web UI server", Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				cfg, err := webui.LoadConfig(env.Getenv)
				if err != nil {
					return err
				}
				return webui.Run(cmd.Context(), cfg, web.FS(), logging.New(env.Stdout, cfg.LogLevel, cfg.LogFormat))
			},
		},
		&cobra.Command{
			Use: "migrate", Short: "Apply database migrations", Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				cfg, err := config.Load(env.Getenv)
				if err != nil {
					return err
				}
				_, err = bootstrap(cmd.Context(), cfg.DatabaseURL, nil)
				return err
			},
		},
		&cobra.Command{
			Use: "version", Short: "Print the version", Args: cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				_, err := fmt.Fprintf(env.Stdout, "werbot %s (commit %s, built %s)\n", version.Version, version.Commit, version.BuildDate)
				return err
			},
		},
		&cobra.Command{
			Use: "healthcheck", Short: "Probe /healthz of the local server", Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return healthcheck(cmd.Context(), cmp.Or(env.Getenv("WERBOT_HTTP_ADDR"), ":8080"))
			},
		},
	)
	root.AddCommand(cli.Commands(env)...)
	return root
}

func runServer(ctx context.Context, args []string, env cli.Env) error {
	var withGateway bool
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.BoolVar(&withGateway, "with-gateway", false, `also run an instance of the system gateway pool "local"`)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %q", fs.Args())
	}

	cfg, err := config.Load(env.Getenv)
	if err != nil {
		return err
	}
	logger := logging.New(env.Stdout, cfg.LogLevel, cfg.LogFormat)
	box, err := secretbox.New(cfg.SecretKey, cfg.PreviousSecretKeys)
	if err != nil {
		return err
	}
	boot, err := bootstrap(ctx, cfg.DatabaseURL, box)
	if err != nil {
		return err
	}
	octx, cancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := db.Open(octx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer pool.Close()
	return serve(ctx, cfg, logger, pool, sdk.Modules(), web.FS(), serveOpts{CA: boot.CA, Signer: boot.Signer, Box: box, WithGateway: withGateway, Getenv: env.Getenv})
}
