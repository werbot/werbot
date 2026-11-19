package policy

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/policy/dbq"
)

// GeoIPRefreshWorker downloads the country database into Postgres.
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

// Work stores this month's database. A URL with "%s" falls back to last
// month's when this month's is not published yet; a fixed URL is fetched once
// a month. Any failure before the commit keeps the stored one.
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
	data, err := w.download(ctx, w.monthURL(month))
	if errors.Is(err, errNotFound) && strings.Contains(w.cfg.GeoIPURL, "%s") {
		month = cur.AddDate(0, -1, 0)
		data, err = w.download(ctx, w.monthURL(month))
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

// monthURL puts month in place of "%s"; Sprintf would mangle "%XX" escapes.
func (w *GeoIPRefreshWorker) monthURL(month time.Time) string {
	return strings.Replace(w.cfg.GeoIPURL, "%s", month.Format("2006-01"), 1)
}

// download fetches and unpacks the database at u. Errors leave u out: it may
// hold a token.
func (w *GeoIPRefreshWorker) download(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("geoip: %w", withoutURL(err))
	}
	resp, err := w.cfg.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geoip: %w", withoutURL(err))
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, errNotFound
	default:
		return nil, fmt.Errorf("geoip: status %d", resp.StatusCode)
	}
	data, err := unpack(resp.Body, w.cfg.GeoIPMaxBytes)
	if err != nil {
		return nil, fmt.Errorf("geoip: %w", err)
	}
	return data, nil
}

// withoutURL drops the URL that *url.Error puts in its message.
func withoutURL(err error) error {
	if ue, ok := errors.AsType[*url.Error](err); ok {
		return ue.Err
	}
	return err
}

// unpack returns the mmdb in body: raw, gzip, or the first regular .mmdb
// file of a tar archive (MaxMind serves tar.gz), at most limit bytes.
func unpack(body io.Reader, limit int64) ([]byte, error) {
	r := bufio.NewReader(body)
	if magic, _ := r.Peek(2); string(magic) == "\x1f\x8b" {
		gz, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		r = bufio.NewReader(gz)
	}
	if h, _ := r.Peek(262); len(h) == 262 && string(h[257:]) == "ustar" {
		tr := tar.NewReader(r)
		for {
			hdr, err := tr.Next()
			if errors.Is(err, io.EOF) {
				return nil, errors.New("no .mmdb file in the tar archive")
			}
			if err != nil {
				return nil, err
			}
			if hdr.Typeflag == tar.TypeReg && strings.HasSuffix(hdr.Name, ".mmdb") {
				return readMax(tr, limit)
			}
		}
	}
	return readMax(r, limit)
}

// readMax reads r whole, failing past limit bytes.
func readMax(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("archive exceeds %d bytes", limit)
	}
	return data, nil
}
