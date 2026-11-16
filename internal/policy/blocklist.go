package policy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/policy/dbq"
)

const (
	blocklistSource = "firehol_level1"
	// maxBlocklistBytes caps the feed (level1 is about 0.3 MiB); a larger one is an error, never truncated.
	maxBlocklistBytes = 32 << 20
)

// internalRanges are never blocked: firehol_level1 lists fullbogons, and
// clients of private networks must not get "Access denied".
var internalRanges = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
}

// BlocklistRefreshWorker downloads the firehol netset into Postgres.
type BlocklistRefreshWorker struct {
	river.WorkerDefaults[BlocklistRefreshArgs]
	svc *Service
	cfg JobsConfig
}

// NewBlocklistRefreshWorker creates the worker.
func NewBlocklistRefreshWorker(s *Service, cfg JobsConfig) *BlocklistRefreshWorker {
	return &BlocklistRefreshWorker{svc: s, cfg: cfg}
}

// Timeout overrides River's 1 minute default, which would cut the download.
func (w *BlocklistRefreshWorker) Timeout(*river.Job[BlocklistRefreshArgs]) time.Duration {
	return jobTimeout(w.cfg)
}

// Work replaces the blocklist; any failure keeps the previous one.
func (w *BlocklistRefreshWorker) Work(ctx context.Context, _ *river.Job[BlocklistRefreshArgs]) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.cfg.BlocklistURL, nil)
	if err != nil {
		return err
	}
	resp, err := w.cfg.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("blocklist: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("blocklist: status %d", resp.StatusCode)
	}
	lr := &io.LimitedReader{R: resp.Body, N: maxBlocklistBytes + 1}
	prefixes, bad, err := parseNetset(lr)
	if err != nil {
		return fmt.Errorf("blocklist: %w", err)
	}
	if lr.N <= 0 {
		return fmt.Errorf("blocklist: feed exceeds %d bytes", maxBlocklistBytes)
	}
	if bad > 0 {
		w.svc.deps.Logger.WarnContext(ctx, "blocklist: unparsable lines", "count", bad)
	}
	if len(prefixes) == 0 {
		return fmt.Errorf("blocklist: no networks in feed")
	}
	return w.svc.ReplaceBlocklist(ctx, blocklistSource, prefixes)
}

// parseNetset reads one address or CIDR per line, skipping comments, blank
// lines and networks that touch internalRanges. bad counts unparsable lines.
func parseNetset(r io.Reader) (prefixes []netip.Prefix, bad int, err error) {
	seen := map[netip.Prefix]bool{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		p, perr := netip.ParsePrefix(line)
		if perr != nil {
			a, aerr := netip.ParseAddr(line)
			if aerr != nil {
				bad++
				continue
			}
			p = netip.PrefixFrom(a, a.BitLen())
		}
		p = p.Masked()
		if seen[p] || overlapsInternal(p) {
			continue
		}
		seen[p] = true
		prefixes = append(prefixes, p)
	}
	return prefixes, bad, sc.Err()
}

func overlapsInternal(p netip.Prefix) bool {
	for _, in := range internalRanges {
		if p.Overlaps(in) {
			return true
		}
	}
	return false
}

// ReplaceBlocklist swaps all networks of source for prefixes in one transaction.
func (s *Service) ReplaceBlocklist(ctx context.Context, source string, prefixes []netip.Prefix) error {
	rows := make([]dbq.ReplaceBlocklistParams, len(prefixes))
	for i, p := range prefixes {
		rows[i] = dbq.ReplaceBlocklistParams{Cidr: p, Source: source}
	}
	return dbx.System(ctx, s.deps.Pool, dbx.ReasonMaintenance, func(tx pgx.Tx) error {
		st := newStore(tx)
		if err := st.DeleteBlocklistSource(ctx, source); err != nil {
			return err
		}
		_, err := st.ReplaceBlocklist(ctx, rows)
		return err
	})
}
