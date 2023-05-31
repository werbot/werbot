package main

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/pkg/fsutil"
)

const (
	typeUpdateMMDB      = "cron:updateMMDB"
	typeUpdateLists     = "cron:updateLists"
	typeDownloadLicense = "license:download"
)

func updateMMDBTask(ctx context.Context, t *asynq.Task) error {
	log.Info().Msg("download mmdb database")
	err := fsutil.Download(
		fmt.Sprintf("%s/GeoLite2-Country.mmdb", internal.GetString("GHOST_DATA", "/data")),
		fmt.Sprintf("%s/GeoLite2-Country.mmdb", internal.GetString("INSTALL_CDN", "https://install.werbot.com")),
	)
	if err != nil {
		log.Error(err).Send()
	}
	return nil
}

func updateHAProxyLists(ctx context.Context, t *asynq.Task) error {
	log.Info().Msg("download haproxy lists")
	lists := []string{"blacklist-agent.txt", "cloudflare-ips.txt"}

	for _, list := range lists {
		err := fsutil.Download(
			fmt.Sprintf("%s/haproxy/%s", internal.GetString("GHOST_DATA", "/data"), list),
			fmt.Sprintf("https://raw.githubusercontent.com/werbot/install.werbot.com/main/cfg/haproxy/%s", list),
		)
		if err != nil {
			log.Error(err).Send()
		}
	}
	return nil
}

func downloadLicense(ctx context.Context, t *asynq.Task) error {
	log.Info().Msg("Download license")
	err := fsutil.Download(
		fmt.Sprintf("%s/license.key", internal.GetString("GHOST_DATA", "/data")),
		"https://api.werbot.com/", // TODO: download license
	)
	if err != nil {
		log.Error(err).Send()
	}
	return nil
}
