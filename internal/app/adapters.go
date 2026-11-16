package app

import (
	"context"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/access"
	_ "github.com/werbot/werbot/internal/connector/ssh" // registers the ssh resource type
	"github.com/werbot/werbot/internal/credential"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/policy"
)

// feedsConfig is where the feed jobs download; tests point it at a closed port.
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
			ResourceID: b.ResourceID, LabelSelector: b.LabelSelector}
	}
	return out, nil
}

// teamsFunc adapts a function to policy.TeamSource.
type teamsFunc func(ctx context.Context, orgID, userID uuid.UUID) ([]uuid.UUID, error)

func (f teamsFunc) UserTeams(ctx context.Context, orgID, userID uuid.UUID) ([]uuid.UUID, error) {
	return f(ctx, orgID, userID)
}

// domains are the resource, credential, access and policy services.
type domains struct {
	inv  *inventory.Service
	cred *credential.Service
	acc  *access.Service
	pol  *policy.Service
}

// newDomains wires the services and loads the stored GeoIP database, so every
// replica decides country rules from start. A broken database is logged, not fatal.
func newDomains(ctx context.Context, pool *pgxpool.Pool, box *secretbox.Box, teams policy.TeamSource, logger *slog.Logger) domains {
	d := domains{inv: inventory.New(pool), cred: credential.New(pool, box), acc: access.New(pool)}
	d.pol = policy.New(policy.Deps{Pool: pool, Resources: policyResources{d.inv}, Bindings: policyBindings{d.acc},
		Teams: teams, Logger: logger})
	if err := d.pol.ReloadGeoIP(ctx); err != nil {
		logger.Warn("geoip reload failed", "err", err)
	}
	return d
}

// mount registers ResourceService, AccessService and PolicyService on mux.
func (d domains) mount(mux *http.ServeMux, opts ...connect.HandlerOption) {
	mux.Handle(werbotv1connect.NewResourceServiceHandler(inventory.NewHandler(d.inv, d.cred), opts...))
	mux.Handle(werbotv1connect.NewAccessServiceHandler(access.NewHandler(d.acc), opts...))
	mux.Handle(werbotv1connect.NewPolicyServiceHandler(policy.NewHandler(d.pol), opts...))
}
