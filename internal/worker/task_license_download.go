package worker

import (
	"context"
	"fmt"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/pkg/utils/fsutil"
	"github.com/werbot/werbot/pkg/worker"
)

const (
	TaskDownloadLicense = worker.TaskPattern("license:download")
)

func downloadLicense() worker.TaskHandler {
	return func(_ context.Context, _ []byte) error {
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
}
