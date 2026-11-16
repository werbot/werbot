package policy

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/policy/dbq"
)

// GeoIPRefreshWorker downloads the DB-IP country database into Postgres.
type GeoIPRefreshWorker struct {
	river.WorkerDefaults[GeoIPRefreshArgs]
	svc *Service
	cfg JobsConfig
}

// NewGeoIPRefreshWorker creates the worker.
func NewGeoIPRefreshWorker(s *Service, cfg JobsConfig) *GeoIPRefreshWorker {
	return &GeoIPRefreshWorker{svc: s, cfg: cfg}
}

// Timeout overrides River's 1 minute default, which would cut the download.
func (w *GeoIPRefreshWorker) Timeout(*river.Job[GeoIPRefreshArgs]) time.Duration {
	return jobTimeout(w.cfg)
}

// Work stores this month's database, or last month's when DB-IP has not
// published it yet. Any failure before the commit keeps the stored one.
func (w *GeoIPRefreshWorker) Work(ctx context.Context, _ *river.Job[GeoIPRefreshArgs]) error {
	now := w.cfg.Now().UTC()
	cur := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	var stored pgtype.Date
	err := dbx.System(ctx, w.svc.deps.Pool, dbx.ReasonMaintenance, func(tx pgx.Tx) (err error) {
		stored, err = newStore(tx).GetGeoIPMonth(ctx)
		return err
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil && stored.Time.Equal(cur) {
		return w.svc.ReloadGeoIP(ctx) // retries a reload that failed after an earlier commit
	}

	month := cur
	data, err := w.download(ctx, month)
	if errors.Is(err, errNotFound) {
		month = cur.AddDate(0, -1, 0)
		data, err = w.download(ctx, month)
	}
	if err != nil {
		return err
	}
	if _, err := OpenGeoIP(data); err != nil {
		return fmt.Errorf("geoip: %w", err)
	}

	err = dbx.System(ctx, w.svc.deps.Pool, dbx.ReasonMaintenance, func(tx pgx.Tx) error {
		st := newStore(tx)
		if err := st.UpsertGeoIP(ctx, dbq.UpsertGeoIPParams{Month: pgtype.Date{Time: month, Valid: true}, Data: data}); err != nil {
			return err
		}
		return st.NotifyGeoIPChanged(ctx)
	})
	if err != nil {
		return err
	}
	return w.svc.ReloadGeoIP(ctx)
}

var errNotFound = errors.New("geoip: not published")

// download fetches and unpacks the database of month.
func (w *GeoIPRefreshWorker) download(ctx context.Context, month time.Time) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(w.cfg.GeoIPURL, month.Format("2006-01")), nil)
	if err != nil {
		return nil, err
	}
	resp, err := w.cfg.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geoip: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, errNotFound
	default:
		return nil, fmt.Errorf("geoip: status %d", resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("geoip: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(gz, w.cfg.GeoIPMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("geoip: %w", err)
	}
	if int64(len(data)) > w.cfg.GeoIPMaxBytes {
		return nil, fmt.Errorf("geoip: archive exceeds %d bytes", w.cfg.GeoIPMaxBytes)
	}
	return data, nil
}
