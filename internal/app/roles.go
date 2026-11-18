package app

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/werbot/werbot/internal/connectcli"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/logging"
	"github.com/werbot/werbot/internal/platform/version"
	"github.com/werbot/werbot/internal/webui"
	"github.com/werbot/werbot/web"
)

const usageText = "usage: werbot <server|gateway|web|connect|migrate|version|healthcheck>"

// runRole runs the commands that need neither the server config nor its
// database: version, healthcheck, web, gateway and connect. ok is false for any other cmd.
func runRole(ctx context.Context, cmd string, args []string, getenv func(string) string, stdout io.Writer) (ok bool, err error) {
	switch cmd {
	case "version":
		_, err := fmt.Fprintf(stdout, "werbot %s (commit %s, built %s)\n", version.Version, version.Commit, version.BuildDate)
		return true, err
	case "healthcheck":
		return true, healthcheck(ctx, cmp.Or(getenv("WERBOT_HTTP_ADDR"), ":8080"))
	case "web":
		cfg, err := webui.LoadConfig(getenv)
		if err != nil {
			return true, err
		}
		return true, webui.Run(ctx, cfg, web.FS(), logging.New(stdout, cfg.LogLevel, cfg.LogFormat))
	case "gateway":
		gcfg, err := gateway.LoadConfig(args[1:], getenv)
		if err != nil {
			return true, err
		}
		level, format, err := config.LoadLogging(getenv)
		if err != nil {
			return true, err
		}
		return true, gateway.Run(ctx, gcfg, logging.New(stdout, level, format))
	case "connect":
		cfg, err := connectcli.LoadConfig(args[1:], getenv)
		if err != nil {
			return true, err
		}
		return true, connectcli.Run(ctx, cfg, stdout, os.Stderr)
	}
	return false, nil
}
