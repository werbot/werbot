package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/hibiken/asynq"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/pkg/fsutil"
	"github.com/werbot/werbot/pkg/logger"
)

var log = logger.New("ghost")

const (
	// TypeUpdateMMDB is ...
	TypeUpdateMMDB = "cron:updateMMDB"

	// TypeUpdateLists is ...
	TypeUpdateLists = "cron:updateLists"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	internal.LoadConfig("../../.env")

	log.Info().Msg("Start ghost server")

	// ----------------------------------------
	location, _ := time.LoadLocation("UTC")
	scheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{
			Addr:     internal.GetString("REDIS_ADDR", "localhost:6379"),
			Password: internal.GetString("REDIS_PASSWORD", "redisPassword"),
			DB:       1,
		},
		&asynq.SchedulerOpts{
			LogLevel: asynq.FatalLevel,
			Location: location,
		},
	)

	// update MMDB database
	if _, err := scheduler.Register("@daily", asynq.NewTask(TypeUpdateMMDB, nil)); err != nil {
		log.Error(err).Send()
	}

	// update HAProxy lists
	if _, err := scheduler.Register("@weekly", asynq.NewTask(TypeUpdateMMDB, nil)); err != nil {
		log.Error(err).Send()
	}

	if err := scheduler.Start(); err != nil {
		log.Error(err).Send()
	}

	// ----------------------------------------
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     internal.GetString("REDIS_ADDR", "localhost:6379"),
			Password: internal.GetString("REDIS_PASSWORD", "redisPassword"),
			DB:       1,
		},
		asynq.Config{
			LogLevel:    asynq.FatalLevel,
			Concurrency: 10,
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeUpdateMMDB, downloadMMDBTask)
	mux.HandleFunc(TypeUpdateLists, downloadHAProxyLists)

	if err := srv.Run(mux); err != nil {
		log.Error(err).Send()
	}
}

func downloadMMDBTask(ctx context.Context, t *asynq.Task) error {
	log.Info().Msg("download mmdb database")
	err := fsutil.Download(
		fmt.Sprintf("%s/GeoLite2-Country.mmdb", internal.GetString("APP_DATA_FOLDER", "/data")),
		fmt.Sprintf("%s/GeoLite2-Country.mmdb", internal.GetString("APP_CDN", "https://cdn.werbot.com")),
	)
	if err != nil {
		log.Error(err).Send()
	}
	return nil
}

func downloadHAProxyLists(ctx context.Context, t *asynq.Task) error {
	log.Info().Msg("download haproxy lists")
	lists := []string{"blacklist-agent.txt", "cloudflare-ips.txt"}

	for _, list := range lists {
		err := fsutil.Download(
			fmt.Sprintf("%s/haproxy/%s", internal.GetString("APP_DATA_FOLDER", "/data"), list),
			fmt.Sprintf("https://raw.githubusercontent.com/werbot/installation/main/core/haproxy/%s", list),
		)
		if err != nil {
			log.Error(err).Send()
		}
	}
	return nil
}

func downloadLicense() {
	log.Info().Msg("download license")
	err := fsutil.Download(
		fmt.Sprintf("%s/license.key", internal.GetString("APP_DATA_FOLDER", "/data")),
		"https://api.werbot.com/", // TODO: down license
	)
	if err != nil {
		log.Error(err).Send()
	}
}
