package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/audit"
	"github.com/werbot/werbot/internal/credential"
	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/notify"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/jobs"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/platform/tlsca"
	"github.com/werbot/werbot/internal/policy"
	"github.com/werbot/werbot/internal/session"
	"github.com/werbot/werbot/internal/session/recstore"
)

// feedsConfig is where the feed jobs download; tests point it at a closed
// port. An empty GeoIPURL takes cfg.GeoIPURL.
var feedsConfig = policy.DefaultJobsConfig()

// policyResources is the inventory as a policy.ResourceSource.
type policyResources struct{ inv *inventory.Service }

func (p policyResources) PolicyResources(ctx context.Context, orgID uuid.UUID) ([]policy.SnapshotResource, error) {
	rs, err := p.inv.List(ctx, orgID, nil)
	if err != nil {
		return nil, err
	}
	out := make([]policy.SnapshotResource, len(rs))
	for i, r := range rs {
		out[i] = policy.SnapshotResource{ID: r.ID, Name: r.Name, Type: r.Type, Labels: r.Labels}
	}
	return out, nil
}

// policyBindings is the access service as a policy.BindingSource.
type policyBindings struct{ acc *access.Service }

func (p policyBindings) PolicyBindings(ctx context.Context, orgID uuid.UUID) ([]policy.Binding, error) {
	bs, err := p.acc.List(ctx, orgID, nil)
	if err != nil {
		return nil, err
	}
	out := make([]policy.Binding, len(bs))
	for i, b := range bs {
		out[i] = policy.Binding{ID: b.ID, SubjectType: b.SubjectType, SubjectID: b.SubjectID,
			ResourceID: b.ResourceID, LabelSelector: b.LabelSelector, Ports: b.Ports}
	}
	return out, nil
}

// teamsFunc adapts a function to policy.TeamSource.
type teamsFunc func(ctx context.Context, orgID, userID uuid.UUID) ([]uuid.UUID, error)

func (f teamsFunc) UserTeams(ctx context.Context, orgID, userID uuid.UUID) ([]uuid.UUID, error) {
	return f(ctx, orgID, userID)
}

// services are the domain services of werbot server and their River client.
type services struct {
	Identity   *identity.Service
	Inventory  *inventory.Service
	Credential *credential.Service
	Access     *access.Service
	Policy     *policy.Service
	Box        *secretbox.Box
	CA         *tlsca.CA
	Gateways   *inventory.Gateways
	Sessions   *session.Service
	Recordings *session.Recordings
	Audit      *audit.Service
	Logger     *slog.Logger // replay errors; nil is slog.Default()
	Jobs       *river.Client[pgx.Tx]
}

// newServices wires the services, registers every worker and periodic job on
// one River client (not started) and loads the stored GeoIP database, so every
// replica decides country rules from start. A broken database is logged, not fatal.
// signer comes from bootstrap; recordings go to cfg.RecordingsS3 or cfg.DataDir;
// reg is the registry /metrics serves.
func newServices(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, box *secretbox.Box, ca *tlsca.CA, signer *session.Signer, reg *metrics.Registry, logger *slog.Logger) (*services, error) {
	mailer, err := notify.NewMailer(cfg.SMTP, cfg.Env == "development", logger)
	if err != nil {
		return nil, err
	}
	store, kind, err := recstore.Open(ctx, cfg.RecordingsS3, cfg.DataDir)
	if err != nil {
		return nil, err
	}
	s := &services{Inventory: inventory.New(pool), Credential: credential.New(pool, box),
		Box: box, CA: ca, Gateways: inventory.NewGateways(pool, ca, box), Sessions: session.New(pool),
		Recordings: session.NewRecordings(pool, store, kind, signer), Audit: audit.New(audit.NewPostgres(pool)), Logger: logger}
	s.Access = access.New(pool, accessEmails{s}, accessSettings{s}, access.Config{WebURL: cfg.WebURL})
	// identity needs the River client, which needs the policy jobs: teams
	// reads s.Identity, which is set below, before any request is served.
	s.Policy = policy.New(policy.Deps{Pool: pool, Resources: policyResources{s.Inventory}, Bindings: policyBindings{s.Access},
		Teams: teamsFunc(func(ctx context.Context, orgID, userID uuid.UUID) ([]uuid.UUID, error) {
			return s.Identity.UserTeams(ctx, orgID, userID)
		}), Logger: logger})
	if err := s.Policy.ReloadGeoIP(ctx); err != nil {
		logger.Warn("geoip reload failed", "err", err)
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, notify.NewWorker(mailer, box))
	feeds := feedsConfig
	if feeds.GeoIPURL == "" {
		feeds.GeoIPURL = cfg.GeoIPURL
	}
	periodic := policy.RegisterJobs(workers, s.Policy, feeds)
	periodic = append(periodic, session.RegisterJobs(workers, s.Sessions, time.Now())...)
	periodic = append(periodic, inventory.RegisterGatewayJobs(workers, s.Gateways)...)
	periodic = append(periodic, access.RegisterApprovalJobs(workers, s.Access)...)
	if s.Jobs, err = jobs.NewClient(pool, workers, periodic, logger); err != nil {
		return nil, err
	}
	s.Identity = identity.New(pool, notify.NewQueue(s.Jobs, box), identity.Config{
		WebURL: cfg.WebURL, SecureCookies: cfg.Env != "development", Logger: logger, Box: box, Metrics: reg,
	})
	return s, nil
}

// mount registers ResourceService, AccessService, ApprovalPolicyService, AccessRequestService, LockService,
// PolicyService, SessionService, AuditService and the recording replay route on mux.
func (s *services) mount(mux *http.ServeMux, opts ...connect.HandlerOption) {
	mux.Handle(werbotv1connect.NewResourceServiceHandler(inventory.NewHandler(s.Inventory, s.Credential, s.Policy, s.Gateways, s.Access), opts...))
	mux.Handle(werbotv1connect.NewAccessServiceHandler(access.NewHandler(s.Access), opts...))
	mux.Handle(werbotv1connect.NewApprovalPolicyServiceHandler(access.NewApprovalPolicyHandler(s.Access), opts...))
	mux.Handle(werbotv1connect.NewAccessRequestServiceHandler(access.NewAccessRequestHandler(s.Access), opts...))
	mux.Handle(werbotv1connect.NewLockServiceHandler(access.NewLockHandler(s.Access), opts...))
	mux.Handle(werbotv1connect.NewPolicyServiceHandler(policy.NewHandler(s.Policy), opts...))
	mux.Handle(werbotv1connect.NewSessionServiceHandler(session.NewSessionHandler(s.Sessions, s.Recordings), opts...))
	mux.Handle(werbotv1connect.NewAuditServiceHandler(audit.NewHandler(s.Audit), opts...))
	mux.Handle("GET /api/recordings/{file}", session.NewReplayHandler(s.Sessions, s.Recordings, s.Logger))
}
