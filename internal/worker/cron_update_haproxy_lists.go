package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/pkg/utils/fsutil"
	"github.com/werbot/werbot/pkg/worker"
)

func cronUpdateHAProxyLists() worker.CronHandler {
	return func(_ context.Context) error {
		log.Info().Msg("Download haproxy lists")
		lists := []string{"blacklist-agent.txt", "cloudflare-ips.txt"}
		var errs []error

		for _, list := range lists {
			err := fsutil.Download(
				fmt.Sprintf("%s/haproxy/%s", internal.GetString("GHOST_DATA", "/data"), list),
				fmt.Sprintf("https://raw.githubusercontent.com/werbot/install.werbot.com/main/cfg/haproxy/%s", list),
			)
			if err != nil {
				log.Error(err).Send()
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}
}
