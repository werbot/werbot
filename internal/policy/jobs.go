package policy

import (
	"net/http"
	"time"

	"github.com/riverqueue/river"

	"github.com/werbot/werbot/internal/platform/jobs"
)

// BlocklistRefreshArgs is the daily firehol download.
type BlocklistRefreshArgs struct{}

// Kind implements river.JobArgs.
func (BlocklistRefreshArgs) Kind() string { return "blocklist.refresh" }

// InsertOpts runs the job in the feeds queue.
func (BlocklistRefreshArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.QueueFeeds}
}

// GeoIPRefreshArgs is the monthly DB-IP download.
type GeoIPRefreshArgs struct{}

// Kind implements river.JobArgs.
func (GeoIPRefreshArgs) Kind() string { return "geoip.refresh" }

// InsertOpts runs the job in the feeds queue.
func (GeoIPRefreshArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.QueueFeeds}
}

// JobsConfig is where and how the feed jobs download.
type JobsConfig struct {
	BlocklistURL  string // firehol netset
	GeoIPURL      string // DB-IP file; %s is the month as 2006-01
	GeoIPMaxBytes int64  // limit of the decompressed database
	HTTP          *http.Client
	Now           func() time.Time
}

// DefaultJobsConfig is the production configuration.
func DefaultJobsConfig() JobsConfig {
	return JobsConfig{
		BlocklistURL:  "https://raw.githubusercontent.com/firehol/blocklist-ipsets/master/firehol_level1.netset",
		GeoIPURL:      "https://download.db-ip.com/free/dbip-country-lite-%s.mmdb.gz",
		GeoIPMaxBytes: 256 << 20,
		HTTP:          &http.Client{Timeout: 2 * time.Minute},
		Now:           time.Now,
	}
}

// monthly fires on the 1st of every month at 04:00 UTC.
type monthly struct{}

// Next implements river.PeriodicSchedule.
func (monthly) Next(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month()+1, 1, 4, 0, 0, 0, time.UTC)
}

// RegisterJobs adds the feed workers and returns their periodic jobs for
// jobs.NewClient. Both run once at start.
func RegisterJobs(workers *river.Workers, s *Service, cfg JobsConfig) []*river.PeriodicJob {
	river.AddWorker(workers, NewBlocklistRefreshWorker(s, cfg))
	river.AddWorker(workers, NewGeoIPRefreshWorker(s, cfg))
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(24*time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return BlocklistRefreshArgs{}, nil },
			&river.PeriodicJobOpts{ID: "blocklist.refresh", RunOnStart: true}),
		river.NewPeriodicJob(monthly{},
			func() (river.JobArgs, *river.InsertOpts) { return GeoIPRefreshArgs{}, nil },
			&river.PeriodicJobOpts{ID: "geoip.refresh", RunOnStart: true}),
	}
}
