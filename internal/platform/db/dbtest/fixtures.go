package dbtest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/platform/principal"
)

func insertID(t testing.TB, pool *pgxpool.Pool, sql string, args ...any) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	return id
}

func exec(t testing.TB, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("dbtest: %v", err)
	}
}

// Org inserts an organization with its default settings row.
func Org(t testing.TB, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	return insertID(t, pool, `WITH o AS (INSERT INTO organization (name) VALUES ('test') RETURNING id),
		s AS (INSERT INTO org_setting (org_id) SELECT id FROM o)
		SELECT id FROM o`)
}

// User inserts a user and its membership in orgID with the given role.
func User(t testing.TB, pool *pgxpool.Pool, orgID uuid.UUID, role string) uuid.UUID {
	t.Helper()
	h := strings.ReplaceAll(uuid.NewString(), "-", "")
	id := insertID(t, pool,
		"INSERT INTO users (email, login, name, password_hash) VALUES ($1, $2, 'Test', 'x') RETURNING id",
		h+"@example.com", h)
	exec(t, pool, "INSERT INTO membership (org_id, user_id, role) VALUES ($1, $2, $3)", orgID, id, role)
	return id
}

// Team inserts a team in orgID with the given members (who must belong to orgID).
func Team(t testing.TB, pool *pgxpool.Pool, orgID uuid.UUID, members ...uuid.UUID) uuid.UUID {
	t.Helper()
	id := insertID(t, pool, "INSERT INTO team (org_id, name) VALUES ($1, $2) RETURNING id", orgID, uuid.NewString())
	for _, m := range members {
		exec(t, pool, "INSERT INTO team_member (team_id, org_id, user_id) VALUES ($1, $2, $3)", id, orgID, m)
	}
	return id
}

// Resource inserts an ssh/servers resource with the given labels.
func Resource(t testing.TB, pool *pgxpool.Pool, orgID uuid.UUID, name string, labels map[string]string) uuid.UUID {
	t.Helper()
	id := insertID(t, pool,
		"INSERT INTO resource (org_id, name, type, category, config) VALUES ($1, $2, 'ssh', 'servers', '{}') RETURNING id",
		orgID, name)
	for k, v := range labels {
		exec(t, pool, "INSERT INTO resource_label (resource_id, org_id, key, value) VALUES ($1, $2, $3, $4)", id, orgID, k, v)
	}
	return id
}

// Gateway inserts a pool named name in orgID with a dummy host key (not a sealed key).
func Gateway(t testing.TB, pool *pgxpool.Pool, orgID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	return insertID(t, pool,
		"INSERT INTO gateway (org_id, name, host_key_ciphertext, host_key_key_id) VALUES ($1, $2, '\\x00', 'test') RETURNING id",
		orgID, name)
}

// Instance inserts a never-seen instance of gatewayID with a random certificate serial valid for 24h.
func Instance(t testing.TB, pool *pgxpool.Pool, orgID, gatewayID uuid.UUID) uuid.UUID {
	t.Helper()
	return insertID(t, pool,
		`INSERT INTO gateway_instance (gateway_id, org_id, hostname, cert_serial, cert_expires_at)
		 VALUES ($1, $2, 'test', $3, now() + interval '24 hours') RETURNING id`,
		gatewayID, orgID, strings.ReplaceAll(uuid.NewString(), "-", ""))
}

// Fixture is a seeded organization: an owner, an admin, two members, two
// gateway pools with one instance each and a resource served by GatewayID.
type Fixture struct {
	OrgID, OwnerID, AdminID, MemberID, OtherMemberID uuid.UUID
	// InstanceID is an instance of GatewayID, OtherInstanceID of OtherGatewayID.
	GatewayID, OtherGatewayID, InstanceID, OtherInstanceID, ResourceID uuid.UUID
}

// Seed inserts a new Fixture.
func Seed(t testing.TB, pool *pgxpool.Pool) Fixture {
	t.Helper()
	org := Org(t, pool)
	f := Fixture{
		OrgID:         org,
		OwnerID:       User(t, pool, org, principal.RoleOwner),
		AdminID:       User(t, pool, org, principal.RoleAdmin),
		MemberID:      User(t, pool, org, principal.RoleMember),
		OtherMemberID: User(t, pool, org, principal.RoleMember),
		GatewayID:     Gateway(t, pool, org, "gw-a"),
		ResourceID:    Resource(t, pool, org, "web", nil),
	}
	f.OtherGatewayID = Gateway(t, pool, org, "gw-b")
	f.InstanceID = Instance(t, pool, org, f.GatewayID)
	f.OtherInstanceID = Instance(t, pool, org, f.OtherGatewayID)
	exec(t, pool, "UPDATE resource SET gateway_id = $1 WHERE id = $2", f.GatewayID, f.ResourceID)
	return f
}

// StartSession inserts an open session of userID on ResourceID, served by InstanceID.
func (f Fixture) StartSession(t testing.TB, pool *pgxpool.Pool, userID uuid.UUID) uuid.UUID {
	t.Helper()
	return insertID(t, pool,
		`INSERT INTO session (org_id, user_id, resource_id, gateway_id, gateway_instance_id, client_ip, kind)
		 VALUES ($1, $2, $3, $4, $5, '10.0.0.1', 'terminal') RETURNING id`,
		f.OrgID, userID, f.ResourceID, f.GatewayID, f.InstanceID)
}

// OnlineInstance marks instanceID seen now, serving the given connector types.
func (f Fixture) OnlineInstance(t testing.TB, pool *pgxpool.Pool, instanceID uuid.UUID, connectors ...string) {
	t.Helper()
	exec(t, pool, "UPDATE gateway_instance SET last_seen_at = now(), connectors = $3 WHERE id = $1 AND org_id = $2",
		instanceID, f.OrgID, append([]string{}, connectors...))
}

// Principal returns userID acting in OrgID with its seeded role.
func (f Fixture) Principal(userID uuid.UUID) principal.Principal {
	role := principal.RoleMember
	switch userID {
	case f.OwnerID:
		role = principal.RoleOwner
	case f.AdminID:
		role = principal.RoleAdmin
	}
	return principal.Principal{UserID: userID, OrgID: f.OrgID, Role: role}
}

// ApprovalPolicy inserts a policy of OrgID; selector, requesters and approvers are jsonb texts.
func (f Fixture) ApprovalPolicy(t testing.TB, pool *pgxpool.Pool, name, mode, selector, requesters, approvers string) uuid.UUID {
	t.Helper()
	return insertID(t, pool,
		`INSERT INTO approval_policy (org_id, name, mode, resource_selector, requesters, approvers)
		 VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6::jsonb) RETURNING id`,
		f.OrgID, name, mode, selector, requesters, approvers)
}

// Grant inserts a grant of resourceID to userID that expires after ttl (a negative ttl gives an expired grant).
func (f Fixture) Grant(t testing.TB, pool *pgxpool.Pool, userID, resourceID uuid.UUID, ttl time.Duration) uuid.UUID {
	t.Helper()
	return insertID(t, pool,
		`INSERT INTO access_grant (org_id, user_id, resource_id, expires_at)
		 VALUES ($1, $2, $3, now() + make_interval(secs => $4::float8)) RETURNING id`,
		f.OrgID, userID, resourceID, ttl.Seconds())
}

// Lock inserts a lock of OrgID with reason "test"; targetID is nil only for all_agents.
func (f Fixture) Lock(t testing.TB, pool *pgxpool.Pool, targetType string, targetID *uuid.UUID) uuid.UUID {
	t.Helper()
	return insertID(t, pool,
		"INSERT INTO lock (org_id, target_type, target_id, reason) VALUES ($1, $2, $3, 'test') RETURNING id",
		f.OrgID, targetType, targetID)
}

// Agent inserts an active agent of sponsorID (a member of orgID) with empty limits.
func Agent(t testing.TB, pool *pgxpool.Pool, orgID, sponsorID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	return insertID(t, pool,
		"INSERT INTO agent (org_id, sponsor_user_id, name, created_by) VALUES ($1, $2, $3, $2) RETURNING id",
		orgID, sponsorID, name)
}

// AgentRun inserts a running run of agentID, owned by the agent's sponsor, expiring in an hour.
func AgentRun(t testing.TB, pool *pgxpool.Pool, orgID, agentID uuid.UUID, maxActions, maxConcurrent int) uuid.UUID {
	t.Helper()
	return insertID(t, pool,
		`INSERT INTO agent_run (org_id, agent_id, sponsor_user_id, client, status, max_actions, max_concurrent, expires_at)
		 SELECT org_id, id, sponsor_user_id, 'test', 'running', $3, $4, now() + interval '1 hour'
		 FROM agent WHERE id = $2 AND org_id = $1 RETURNING id`,
		orgID, agentID, maxActions, maxConcurrent)
}
