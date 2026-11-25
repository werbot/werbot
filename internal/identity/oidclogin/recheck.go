package oidclogin

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/platform/jobs"
)

// idp.recheck runs every RecheckInterval over up to RecheckBatch links not
// checked for RecheckAfter: an IdP refusal ends the user's sessions within 1 h.
const (
	RecheckInterval = 10 * time.Minute
	RecheckAfter    = 50 * time.Minute
	RecheckBatch    = 100
)

// ErrIdPRejected means an IdP refused a user's refresh token (invalid_grant).
var ErrIdPRejected = errors.New("identity provider rejected the user") //nolint:revive // name fixed by contract

// Recheck claims up to RecheckBatch links not checked for RecheckAfter and
// refreshes them at their IdPs (the idp.recheck job). A new refresh token is
// stored sealed; an explicit invalid_grant revokes the user's sessions; an
// unreachable IdP (network, timeout, 5xx, discovery) keeps the sessions and
// its links wait for the next window. It returns store errors.
// ponytail: one batch per run (600 links/h); loop over batches if installs outgrow it.
func Recheck(ctx context.Context, store IdPStore, s *identity.Service, cfg Config) error {
	links, err := store.StaleLinks(ctx, RecheckAfter, RecheckBatch)
	if err != nil {
		return err
	}
	c := newChecker(store, s, cfg)
	var errs []error
	for _, l := range links {
		_, err := c.check(ctx, l)
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// RecheckUser claims and refreshes the links of userID in orgID not checked
// for RecheckAfter. It returns ErrIdPRejected once an IdP refused one (the
// user's sessions are revoked by then); an unreachable IdP and store errors
// are logged and give nil, so a CLI renewal proceeds.
func RecheckUser(ctx context.Context, store IdPStore, s *identity.Service, cfg Config, orgID, userID uuid.UUID) error {
	links, err := store.UserStaleLinks(ctx, orgID, userID, RecheckAfter)
	if err != nil {
		slog.Default().Warn("idp recheck: links", "org_id", orgID, "user_id", userID, "err", err)
		return nil
	}
	c := newChecker(store, s, cfg)
	for _, l := range links {
		rejected, err := c.check(ctx, l)
		if err != nil {
			slog.Default().Warn("idp recheck: store", "idp_id", l.IdPID, "user_id", l.UserID, "err", err)
		}
		if rejected {
			return ErrIdPRejected
		}
	}
	return nil
}

// checker refreshes links; each IdP is looked up and discovered once.
type checker struct {
	store   IdPStore
	svc     *identity.Service
	cfg     Config
	parties map[uuid.UUID]party
}

type party struct {
	rp  rp.RelyingParty
	err error
}

func newChecker(store IdPStore, s *identity.Service, cfg Config) *checker {
	return &checker{store: store, svc: s, cfg: cfg, parties: map[uuid.UUID]party{}}
}

// check refreshes l at its IdP. rejected reports an invalid_grant (the
// user's sessions revoked unless err says otherwise); an unreachable IdP is
// logged and gives neither; err is a store error.
func (c *checker) check(ctx context.Context, l Link) (rejected bool, err error) {
	p, err := c.party(ctx, l.IdPID)
	if err != nil {
		unreachable(l, err)
		return false, nil
	}
	tokens, err := rp.RefreshTokens[*oidc.IDTokenClaims](ctx, p, l.Refresh, "", "")
	if err == nil {
		return false, c.store.UpdateLink(ctx, l, optional(tokens.RefreshToken), time.Now())
	}
	if oe, ok := errors.AsType[*oidc.Error](err); ok && oe.ErrorType == oidc.InvalidGrant {
		return true, c.revoke(ctx, l)
	}
	unreachable(l, err)
	return false, nil
}

// unreachable logs why the IdP of l gave no answer (never a token).
func unreachable(l Link, err error) {
	slog.Default().Warn("idp recheck: identity provider unreachable", "idp_id", l.IdPID, "err", err)
}

// revoke ends the sessions of l's user, then drops the refused token with
// the audit; a failure after the first step is retried by the next run,
// which still finds the token.
func (c *checker) revoke(ctx context.Context, l Link) error {
	if err := c.svc.RevokeUserSessions(ctx, l.OrgID, l.UserID, "idp"); err != nil {
		return err
	}
	return c.store.RevokeLink(ctx, l)
}

// party is the relying party of IdP id, discovered on first use.
func (c *checker) party(ctx context.Context, id uuid.UUID) (rp.RelyingParty, error) {
	p, ok := c.parties[id]
	if !ok {
		var idp IdP
		if idp, p.err = c.store.Get(ctx, id); p.err == nil {
			p.rp, p.err = relyingParty(ctx, c.cfg, idp)
		}
		c.parties[id] = p
	}
	return p.rp, p.err
}

// optional is nil for "": the IdP issued no new refresh token, keep the stored one.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// RecheckArgs is the idp.recheck job; it carries nothing.
type RecheckArgs struct{}

// Kind implements river.JobArgs.
func (RecheckArgs) Kind() string { return "idp.recheck" }

// InsertOpts runs the job in the idp queue.
func (RecheckArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{Queue: jobs.QueueIdP} }

type recheckWorker struct {
	river.WorkerDefaults[RecheckArgs]
	store IdPStore
	svc   *identity.Service
	cfg   Config
}

func (w *recheckWorker) Work(ctx context.Context, _ *river.Job[RecheckArgs]) error {
	return Recheck(ctx, w.store, w.svc, w.cfg)
}

// RegisterJobs adds the idp.recheck worker and returns its periodic job
// (every RecheckInterval, run by the River leader) for jobs.NewClient.
func RegisterJobs(workers *river.Workers, store IdPStore, s *identity.Service, cfg Config) []*river.PeriodicJob {
	river.AddWorker(workers, &recheckWorker{store: store, svc: s, cfg: cfg})
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(RecheckInterval),
			func() (river.JobArgs, *river.InsertOpts) { return RecheckArgs{}, nil },
			&river.PeriodicJobOpts{ID: "idp.recheck"}),
	}
}
