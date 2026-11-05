package factory

import (
	"encoding/json"
	"time"

	"github.com/stretchr/testify/require"
)

// SchemeParams is the set of optional and required values for a scheme.
type SchemeParams struct {
	ID, ProjectID, Title, Description string
	SchemeType                        int16
	Access, AccessPolicy              map[string]any
	Active, Audit, Online             *bool
	LockedAt, ArchivedAt, CreatedAt   *time.Time
}

// Scheme is a created scheme row.
type Scheme struct {
	ID, ProjectID, Title string
	SchemeType           int16
	Access               map[string]any
}

// Scheme inserts one scheme row and returns it.
func (b *Builder) Scheme(p SchemeParams) Scheme {
	b.t.Helper()

	p.ID = uuidOrNew(p.ID)
	if p.Title == "" {
		p.Title = "scheme"
	}
	if p.SchemeType == 0 {
		p.SchemeType = 103
	}
	active := boolOr(p.Active, true)
	audit := boolOr(p.Audit, false)
	online := boolOr(p.Online, false)

	if p.Access == nil {
		p.Access = map[string]any{
			"alias":   randomAlias(),
			"address": "127.0.0.1",
			"port":    22,
			"password": map[string]any{
				"login":    "user",
				"password": defaultPassword,
			},
		}
	}
	if p.AccessPolicy == nil {
		p.AccessPolicy = map[string]any{"country": 0, "network": 0}
	}

	access, err := json.Marshal(p.Access)
	require.NoError(b.t, err, "scheme: marshal access")
	accessPolicy, err := json.Marshal(p.AccessPolicy)
	require.NoError(b.t, err, "scheme: marshal access policy")

	_, err = b.db.Exec(`
		INSERT INTO "scheme" (
			"id", "project_id", "title", "description",
			"scheme_type", "access", "access_policy",
			"active", "audit", "online",
			"locked_at", "archived_at", "created_at"
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, COALESCE($13, NOW()))
	`, p.ID, p.ProjectID, p.Title, p.Description,
		p.SchemeType, access, accessPolicy,
		active, audit, online,
		p.LockedAt, p.ArchivedAt, p.CreatedAt)
	require.NoError(b.t, err, "scheme: insert")

	return Scheme{
		ID:         p.ID,
		ProjectID:  p.ProjectID,
		Title:      p.Title,
		SchemeType: p.SchemeType,
		Access:     p.Access,
	}
}

// SchemeMemberParams is the set of optional and required values for a scheme member.
type SchemeMemberParams struct {
	ID, SchemeID, ProjectMemberID              string
	Active, Online                             *bool
	LockedAt, ArchivedAt, UpdatedAt, CreatedAt *time.Time
}

// SchemeMember is a created scheme member row.
type SchemeMember struct {
	ID, SchemeID, ProjectMemberID string
	Active, Online                bool
}

// SchemeMember inserts one scheme member row and returns it.
func (b *Builder) SchemeMember(m SchemeMemberParams) SchemeMember {
	b.t.Helper()

	m.ID = uuidOrNew(m.ID)
	active := boolOr(m.Active, true)
	online := boolOr(m.Online, false)

	_, err := b.db.Exec(`
		INSERT INTO "scheme_member" (
			"id", "scheme_id", "project_member_id", "active", "online",
			"locked_at", "archived_at", "updated_at", "created_at"
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9, NOW()))
	`, m.ID, m.SchemeID, m.ProjectMemberID, active, online,
		m.LockedAt, m.ArchivedAt, m.UpdatedAt, m.CreatedAt)
	require.NoError(b.t, err, "scheme member: insert")

	return SchemeMember{
		ID:              m.ID,
		SchemeID:        m.SchemeID,
		ProjectMemberID: m.ProjectMemberID,
		Active:          active,
		Online:          online,
	}
}

// SchemeActivity sets the activity schedule of a scheme.
func (b *Builder) SchemeActivity(schemeID string, data map[string][]int32) {
	b.t.Helper()

	payload, err := json.Marshal(data)
	require.NoError(b.t, err, "scheme activity: marshal data")

	result, err := b.db.Exec(`
		UPDATE "scheme_activity" SET "data" = $1 WHERE "scheme_id" = $2
	`, payload, schemeID)
	require.NoError(b.t, err, "scheme activity: update")

	affected, err := result.RowsAffected()
	require.NoError(b.t, err, "scheme activity: rows affected")
	if affected > 0 {
		return
	}

	_, err = b.db.Exec(`
		INSERT INTO "scheme_activity" ("scheme_id", "data") VALUES ($1, $2)
	`, schemeID, payload)
	require.NoError(b.t, err, "scheme activity: insert")
}

// SchemeHostKey sets the host key of a scheme.
func (b *Builder) SchemeHostKey(schemeID string, key []byte) {
	b.t.Helper()

	result, err := b.db.Exec(`
		UPDATE "scheme_host_key" SET "host_key" = $1 WHERE "scheme_id" = $2
	`, key, schemeID)
	require.NoError(b.t, err, "scheme host key: update")

	affected, err := result.RowsAffected()
	require.NoError(b.t, err, "scheme host key: rows affected")
	if affected > 0 {
		return
	}

	_, err = b.db.Exec(`
		INSERT INTO "scheme_host_key" ("scheme_id", "host_key") VALUES ($1, $2)
	`, schemeID, key)
	require.NoError(b.t, err, "scheme host key: insert")
}

// SchemeFirewallCountryParams is the set of optional and required values for a scheme firewall country.
type SchemeFirewallCountryParams struct {
	ID, SchemeID, CountryCode string
}

// SchemeFirewallCountry is a created scheme firewall country row.
type SchemeFirewallCountry struct{ ID, SchemeID, CountryCode string }

// SchemeFirewallCountry inserts one scheme firewall country row and returns it.
func (b *Builder) SchemeFirewallCountry(c SchemeFirewallCountryParams) SchemeFirewallCountry {
	b.t.Helper()

	c.ID = uuidOrNew(c.ID)

	_, err := b.db.Exec(`
		INSERT INTO "scheme_firewall_country" ("id", "scheme_id", "country_code")
		VALUES ($1, $2, $3)
	`, c.ID, c.SchemeID, c.CountryCode)
	require.NoError(b.t, err, "scheme firewall country: insert")

	return SchemeFirewallCountry(c)
}

// SchemeFirewallNetworkParams is the set of optional and required values for a scheme firewall network.
type SchemeFirewallNetworkParams struct {
	ID, SchemeID, Network string
}

// SchemeFirewallNetwork is a created scheme firewall network row.
type SchemeFirewallNetwork struct{ ID, SchemeID, Network string }

// SchemeFirewallNetwork inserts one scheme firewall network row and returns it.
func (b *Builder) SchemeFirewallNetwork(n SchemeFirewallNetworkParams) SchemeFirewallNetwork {
	b.t.Helper()

	n.ID = uuidOrNew(n.ID)

	_, err := b.db.Exec(`
		INSERT INTO "scheme_firewall_network" ("id", "scheme_id", "network")
		VALUES ($1, $2, $3)
	`, n.ID, n.SchemeID, n.Network)
	require.NoError(b.t, err, "scheme firewall network: insert")

	return SchemeFirewallNetwork(n)
}
