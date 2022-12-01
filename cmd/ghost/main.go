package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/logger"
	"github.com/werbot/werbot/internal/utils/files"
)

var (
	component = "ghost"
	log       = logger.NewLogger(component)
)

func main() {
	rand.Seed(time.Now().UnixNano())

	internal.LoadConfig("../../.env")

	var flagInit bool
	flag.BoolVar(&flagInit, "init", false, "Initializing the required databases")
	flag.Parse()
	if flagInit {
		log.Info().Msg("Initializing the required databases")
		//downloadMMDB()
		//downloadHAProxyLists()
		downloadLicense()
		return
	}

	log.Info().Msg("Start ghost server")

	utcTime, _ := time.LoadLocation("UTC")
	scheduler := cron.New(cron.WithLocation(utcTime))
	defer scheduler.Stop()

	scheduler.AddFunc("0 0 * * *", downloadMMDB)
	scheduler.AddFunc("5 8 * * 0", downloadHAProxyLists)

	go scheduler.Start()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}

func downloadMMDB() {
	log.Info().Msg("download mmdb")
	err := files.DownloadFile(
		fmt.Sprintf("%s/GeoLite2-Country.mmdb", internal.GetString("APP_DATA_FOLDER", "/data")),
		fmt.Sprintf("%s/GeoLite2-Country.mmdb", internal.GetString("APP_CDN", "https://cdn.werbot.com")),
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to download mmdb")
	}
}

func downloadHAProxyLists() {
	log.Info().Msg("download haproxy lists")
	lists := []string{"blacklist-agent.txt", "cloudflare-ips.txt"}

	for _, list := range lists {
		err := files.DownloadFile(
			fmt.Sprintf("%s/haproxy/%s", internal.GetString("APP_DATA_FOLDER", "/data"), list),
			fmt.Sprintf("https://raw.githubusercontent.com/werbot/installation/main/core/haproxy/%s", list),
		)
		if err != nil {
			log.Error().Err(err).Msg("failed to download mmdb")
		}
	}
}

// TODO: рыба
func downloadLicense() {
	log.Info().Msg("download license")
	err := files.DownloadFile(
		fmt.Sprintf("%s/license.key", internal.GetString("APP_DATA_FOLDER", "/data")),
		"https://api.werbot.com/", // TODO: down license
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to download license")
	}
}
