package inventory

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/inventory/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/secretbox"
)

// Gateway pool limits and defaults.
const (
	CertTTL                 = 24 * time.Hour
	JoinTokenTTLDefault     = 24 * time.Hour
	JoinTokenMinTTL         = time.Hour
	JoinTokenMaxTTL         = 8760 * time.Hour
	JoinTokenMaxUsesDefault = 10
	JoinTokenMaxUsesMax     = 1000
	SystemPoolName          = "local"
	SystemJoinTokenTTL      = time.Hour // system tokens have max_uses 1
)

const maxGatewayName = 63

var errGatewayNotFound = fmt.Errorf("gateway not found: %w", apperr.ErrNotFound)

// CertIssuer signs gateway instance certificates (*tlsca.CA).
type CertIssuer interface {
	IssueClient(csrDER []byte, commonName string, ttl time.Duration) ([]byte, error)
	RootPEM() []byte
	Fingerprint() string
}

// Gateway is a pool of gateway instances sharing one SSH host key.
type Gateway struct {
	ID, OrgID  uuid.UUID
	Name       string
	Online     bool     // at least one instance is online
	Connectors []string // common to every online instance
	CreatedAt  time.Time
	Instances  []Instance // by created_at
}

// Instance is one gateway process of a pool.
type Instance struct {
	ID, GatewayID, OrgID          uuid.UUID
	Hostname, Version, CertSerial string
	Connectors                    []string
	ClientIP                      netip.Addr
	CertExpiresAt, CreatedAt      time.Time
	LastSeenAt                    *time.Time
	Online                        bool
}

// Gateways manages gateway pools, their instances and join tokens.
type Gateways struct {
	pool *pgxpool.Pool
	ca   CertIssuer
	box  *secretbox.Box
}

// NewGateways creates Gateways; box seals the pool host keys.
func NewGateways(pool *pgxpool.Pool, ca CertIssuer, box *secretbox.Box) *Gateways {
	return &Gateways{pool: pool, ca: ca, box: box}
}

// admin runs fn in the caller's organization; only owners and admins pass.
func (g *Gateways) admin(ctx context.Context, fn func(principal.Principal, Store, pgx.Tx) error) error {
	p, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin)
	if err != nil {
		return err
	}
	return dbx.InOrg(ctx, g.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error { return fn(p, newStore(tx), tx) })
}

// actor is who an audit event is attributed to.
type actor struct {
	kind string
	user *uuid.UUID
	ip   netip.Addr
}

func userActor(ctx context.Context, p principal.Principal) actor {
	return actor{kind: "user", user: &p.UserID, ip: clientip.From(ctx)}
}

func (a actor) audit(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, action, targetType string, id uuid.UUID, data map[string]any) error {
	return auditlog.Write(ctx, tx, auditlog.Event{OrgID: &orgID, ActorUserID: a.user, ActorKind: a.kind, Action: action,
		TargetType: targetType, TargetID: id.String(), IP: a.ip, Data: data})
}

// newHostKey returns a fresh ed25519 pool host key in OpenSSH PEM, sealed.
func (g *Gateways) newHostKey() (ciphertext []byte, keyID string, err error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", err
	}
	blk, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		return nil, "", err
	}
	return g.box.Seal(pem.EncodeToMemory(blk))
}

// poolCreated hands unassigned resources to the first pool of the organization and audits the pool.
// The lock waits for transactions that saved a resource without a pool (see pickGateway).
func poolCreated(ctx context.Context, st Store, tx pgx.Tx, a actor, orgID, id uuid.UUID, name string) error {
	if err := st.LockOrgPools(ctx, orgID); err != nil {
		return err
	}
	if _, err := st.AssignUnpooledResources(ctx, dbq.AssignUnpooledResourcesParams{GatewayID: id, OrgID: orgID}); err != nil {
		return err
	}
	return a.audit(ctx, tx, orgID, "gateway.create", "gateway", id, map[string]any{"name": name})
}

// Create adds a pool with a new host key. The first pool of the organization
// takes every resource without one.
func (g *Gateways) Create(ctx context.Context, name string) (Gateway, error) {
	if n := utf8.RuneCountInString(name); n < 1 || n > maxGatewayName {
		return Gateway{}, invalid("gateway name must be 1 to %d characters", maxGatewayName)
	}
	var out Gateway
	err := g.admin(ctx, func(p principal.Principal, st Store, tx pgx.Tx) error {
		ct, keyID, err := g.newHostKey()
		if err != nil {
			return err
		}
		row, err := st.CreateGateway(ctx, dbq.CreateGatewayParams{OrgID: p.OrgID, Name: name, HostKeyCiphertext: ct, HostKeyKeyID: keyID})
		if isUniqueViolation(err) {
			return fmt.Errorf("gateway name already used: %w", apperr.ErrAlreadyExists)
		}
		if err != nil {
			return err
		}
		out = Gateway{ID: row.ID, OrgID: p.OrgID, Name: name, CreatedAt: row.CreatedAt}
		return poolCreated(ctx, st, tx, userActor(ctx, p), p.OrgID, row.ID, name)
	})
	return out, err
}

func toInstance(r dbq.ListInstancesRow) Instance {
	in := Instance{ID: r.ID, GatewayID: r.GatewayID, OrgID: r.OrgID, Hostname: r.Hostname, Version: r.Version,
		CertSerial: r.CertSerial, Connectors: r.Connectors, CertExpiresAt: r.CertExpiresAt, CreatedAt: r.CreatedAt,
		LastSeenAt: r.LastSeenAt, Online: r.Online}
	if r.ClientIp != nil {
		in.ClientIP = *r.ClientIp
	}
	return in
}

// poolStatus: a pool is online with any online instance; its connectors are
// those every online instance has.
func poolStatus(instances []Instance) (online bool, connectors []string) {
	for _, in := range instances {
		switch {
		case !in.Online:
		case !online:
			online, connectors = true, slices.Clone(in.Connectors)
		default:
			connectors = slices.DeleteFunc(connectors, func(c string) bool { return !slices.Contains(in.Connectors, c) })
		}
	}
	return online, connectors
}

// List returns the pools of the organization by name with their instances.
func (g *Gateways) List(ctx context.Context) ([]Gateway, error) {
	var out []Gateway
	err := g.admin(ctx, func(p principal.Principal, st Store, _ pgx.Tx) error {
		pools, err := st.ListGateways(ctx, p.OrgID)
		if err != nil {
			return err
		}
		rows, err := st.ListInstances(ctx, p.OrgID)
		if err != nil {
			return err
		}
		byPool := map[uuid.UUID][]Instance{}
		for _, r := range rows {
			byPool[r.GatewayID] = append(byPool[r.GatewayID], toInstance(r))
		}
		for _, r := range pools {
			gw := Gateway{ID: r.ID, OrgID: r.OrgID, Name: r.Name, CreatedAt: r.CreatedAt, Instances: byPool[r.ID]}
			gw.Online, gw.Connectors = poolStatus(gw.Instances)
			out = append(out, gw)
		}
		return nil
	})
	return out, err
}

// Delete removes a pool with its instances and join tokens; a pool that
// still serves resources is ErrFailedPrecondition.
func (g *Gateways) Delete(ctx context.Context, id uuid.UUID) error {
	return g.admin(ctx, func(p principal.Principal, st Store, tx pgx.Tx) error {
		n, err := st.DeleteGateway(ctx, dbq.DeleteGatewayParams{ID: id, OrgID: p.OrgID})
		if isConstraintViolation(err, "resource_gateway_fk") { // RESTRICT: SQLSTATE 23001
			return fmt.Errorf("%w: gateway still serves resources", apperr.ErrFailedPrecondition)
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return errGatewayNotFound
		}
		return userActor(ctx, p).audit(ctx, tx, p.OrgID, "gateway.delete", "gateway", id, nil)
	})
}

// DeleteInstance removes one instance; its certificate stops authenticating.
func (g *Gateways) DeleteInstance(ctx context.Context, id uuid.UUID) error {
	return g.admin(ctx, func(p principal.Principal, st Store, tx pgx.Tx) error {
		gatewayID, err := st.DeleteInstance(ctx, dbq.DeleteInstanceParams{ID: id, OrgID: p.OrgID})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("gateway instance not found: %w", apperr.ErrNotFound)
		}
		if err != nil {
			return err
		}
		return userActor(ctx, p).audit(ctx, tx, p.OrgID, "gateway.instance.delete", "gateway_instance", id,
			map[string]any{"gateway_id": gatewayID.String()})
	})
}
