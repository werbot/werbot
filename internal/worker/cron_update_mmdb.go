package worker

import (
	"context"
	"fmt"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/pkg/utils/fsutil"
	"github.com/werbot/werbot/pkg/worker"
)

func cronUpdateMMDB() worker.CronHandler {
	return func(_ context.Context) error {
		log.Info().Msg("Download mmdb database")
		err := fsutil.Download(
			fmt.Sprintf("%s/GeoLite2-Country.mmdb", internal.GetString("GHOST_DATA", "/data")),
			fmt.Sprintf("%s/GeoLite2-Country.mmdb", internal.GetString("INSTALL_CDN", "https://install.werbot.com")),
		)
		if err != nil {
			log.Error(err).Send()
		}
		return nil
	}
}
