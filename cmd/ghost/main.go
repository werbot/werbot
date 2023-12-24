package main

import (
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/version"
	"github.com/werbot/werbot/pkg/fsutil"
	"github.com/werbot/werbot/pkg/logger"
)

var log logger.Logger

func main() {
	godotenv.Load(".env", "/etc/werbot/.env")

	log = logger.New()

	log.Info().Str("version", version.Version()).Msg("Start ghost server")

	asyncRedisConfig := asynq.RedisClientOpt{
		Addr:     internal.GetString("REDIS_ADDR", "localhost:6379"),
		Password: internal.GetString("REDIS_PASSWORD", "redisPassword"),
		DB:       1,
	}

	// server ---
	srv := asynq.NewServer(asyncRedisConfig, asynq.Config{
		LogLevel:    asynq.FatalLevel,
		Concurrency: 10,
	})

	mux := asynq.NewServeMux()
	mux.HandleFunc(typeUpdateMMDB, updateMMDBTask)
	mux.HandleFunc(typeUpdateLists, updateHAProxyLists)
	mux.HandleFunc(typeDownloadLicense, downloadLicense)

	if err := srv.Start(mux); err != nil {
		log.Error(err).Send()
	}

	if !fsutil.FileExists(fmt.Sprintf("%s/license.key", internal.GetString("GHOST_DATA", "/data"))) {
		client := asynq.NewClient(asyncRedisConfig)

		// tasks
		if _, err := client.Enqueue(asynq.NewTask(typeDownloadLicense, nil)); err != nil {
			log.Fatal(err)
		}
		if _, err := client.Enqueue(asynq.NewTask(typeUpdateMMDB, nil)); err != nil {
			log.Fatal(err)
		}
		if _, err := client.Enqueue(asynq.NewTask(typeUpdateLists, nil)); err != nil {
			log.Fatal(err)
		}

		client.Close()
	}

	// scheduler ---
	location, _ := time.LoadLocation("UTC")
	scheduler := asynq.NewScheduler(asyncRedisConfig,
		&asynq.SchedulerOpts{
			LogLevel: asynq.FatalLevel,
			Location: location,
		},
	)

	// update MMDB database
	if _, err := scheduler.Register("@daily", asynq.NewTask(typeUpdateMMDB, nil)); err != nil {
		log.Error(err).Send()
	}

	// update HAProxy lists
	if _, err := scheduler.Register("@weekly", asynq.NewTask(typeUpdateMMDB, nil)); err != nil {
		log.Error(err).Send()
	}

	if err := scheduler.Run(); err != nil {
		log.Error(err).Send()
	}
}
