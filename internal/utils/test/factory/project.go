package factory

import (
	"time"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/pkg/crypto"
)

// ProjectParams is the set of optional and required values for a project.
type ProjectParams struct {
	ID, OwnerID, Title, Alias       string
	LockedAt, ArchivedAt, CreatedAt *time.Time
}

// Project is a created project row.
type Project struct{ ID, OwnerID, Title, Alias string }

// Project inserts one project row and returns it.
func (b *Builder) Project(p ProjectParams) Project {
	b.t.Helper()

	p.ID = uuidOrNew(p.ID)
	if p.Title == "" {
		p.Title = "project"
	}
	if p.Alias == "" {
		p.Alias = randomAlias()
	}

	_, err := b.db.Exec(`
		INSERT INTO "project" (
			"id", "owner_id", "title", "alias",
			"locked_at", "archived_at", "created_at"
		) VALUES ($1, $2, $3, $4, $5, $6, COALESCE($7, NOW()))
	`, p.ID, p.OwnerID, p.Title, p.Alias, p.LockedAt, p.ArchivedAt, p.CreatedAt)
	require.NoError(b.t, err, "project: insert")

	return Project{
		ID:      p.ID,
		OwnerID: p.OwnerID,
		Title:   p.Title,
		Alias:   p.Alias,
	}
}

// ProjectMemberParams is the set of optional and required values for a project member.
type ProjectMemberParams struct {
	ID, ProjectID, ProfileID        string
	Role                            int16
	Active, Online                  *bool
	LockedAt, ArchivedAt, CreatedAt *time.Time
}

// ProjectMember is a created project member row.
type ProjectMember struct {
	ID, ProjectID, ProfileID string
	Role                     int16
	Active, Online           bool
}

// ProjectMember inserts one project member row and returns it.
func (b *Builder) ProjectMember(m ProjectMemberParams) ProjectMember {
	b.t.Helper()

	m.ID = uuidOrNew(m.ID)
	if m.Role == 0 {
		m.Role = 1
	}
	active := boolOr(m.Active, true)
	online := boolOr(m.Online, false)

	_, err := b.db.Exec(`
		INSERT INTO "project_member" (
			"id", "project_id", "profile_id", "role", "active", "online",
			"locked_at", "archived_at", "created_at"
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9, NOW()))
	`, m.ID, m.ProjectID, m.ProfileID, m.Role, active, online,
		m.LockedAt, m.ArchivedAt, m.CreatedAt)
	require.NoError(b.t, err, "project member: insert")

	return ProjectMember{
		ID:        m.ID,
		ProjectID: m.ProjectID,
		ProfileID: m.ProfileID,
		Role:      m.Role,
		Active:    active,
		Online:    online,
	}
}

// ProjectAPIParams is the set of optional and required values for a project API key.
type ProjectAPIParams struct {
	ID, ProjectID, APIKey, APISecret string
	Active                           *bool
	CreatedAt                        *time.Time
}

// ProjectAPI is a created project API key row.
type ProjectAPI struct {
	ID, ProjectID, APIKey, APISecret string
	Active                           bool
}

// ProjectAPI inserts one project API key row and returns it.
func (b *Builder) ProjectAPI(a ProjectAPIParams) ProjectAPI {
	b.t.Helper()

	a.ID = uuidOrNew(a.ID)
	if a.APIKey == "" {
		a.APIKey = crypto.NewPassword(37, false)
	}
	if a.APISecret == "" {
		a.APISecret = crypto.NewPassword(37, false)
	}
	active := boolOr(a.Active, true)

	_, err := b.db.Exec(`
		INSERT INTO "project_api" (
			"id", "project_id", "api_key", "api_secret", "active", "created_at"
		) VALUES ($1, $2, $3, $4, $5, COALESCE($6, NOW()))
	`, a.ID, a.ProjectID, a.APIKey, a.APISecret, active, a.CreatedAt)
	require.NoError(b.t, err, "project API: insert")

	return ProjectAPI{
		ID:        a.ID,
		ProjectID: a.ProjectID,
		APIKey:    a.APIKey,
		APISecret: a.APISecret,
		Active:    active,
	}
}

// ProjectLDAPParams is the set of optional and required values for a project LDAP config.
type ProjectLDAPParams struct {
	ID, ProjectID, Host, Password, RootDN string
	QueryDN, ObjectClass, Description     string
	Port                                  int32
	CreatedAt                             *time.Time
}

// ProjectLDAP is a created project LDAP config row.
type ProjectLDAP struct{ ID, ProjectID string }

// ProjectLDAP inserts one project LDAP config row and returns it.
func (b *Builder) ProjectLDAP(l ProjectLDAPParams) ProjectLDAP {
	b.t.Helper()

	l.ID = uuidOrNew(l.ID)

	_, err := b.db.Exec(`
		INSERT INTO "project_ldap" (
			"id", "project_id", "host", "password", "root_dn",
			"query_dn", "object_class", "description", "port", "created_at"
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, COALESCE($10, NOW()))
	`, l.ID, l.ProjectID, l.Host, l.Password, l.RootDN,
		l.QueryDN, l.ObjectClass, l.Description, l.Port, l.CreatedAt)
	require.NoError(b.t, err, "project LDAP: insert")

	return ProjectLDAP{
		ID:        l.ID,
		ProjectID: l.ProjectID,
	}
}
