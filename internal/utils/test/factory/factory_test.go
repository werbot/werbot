package factory_test

import (
	"database/sql"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/uuid"
)

func newBuilder(t *testing.T) (*factory.Builder, *sql.DB) {
	t.Helper()

	db := test.ServerPostgres(t, "../../../../migration").Conn().Conn
	return factory.New(t, db), db
}

func TestBuilderProfile(t *testing.T) {
	b, db := newBuilder(t)

	profile := b.Profile(factory.ProfileParams{})

	assert.True(t, uuid.IsValid(profile.ID), "profile ID must be a valid UUID")
	assert.Len(t, profile.Alias, 8, "default alias must have 8 characters")
	assert.Equal(t, profile.Alias, regexp.MustCompile(`^[a-z]{8}$`).FindString(profile.Alias), "default alias must be lowercase")
	assert.Equal(t, int32(1), profile.Role, "default role must be 1")
	assert.True(t, profile.Active, "profile must be active by default")
	assert.True(t, profile.Confirmed, "profile must be confirmed by default")
	assert.True(t, crypto.CheckPasswordHash("password", profile.Password), "default password hash must verify")

	var (
		alias, name, surname, email, password string
		role                                  int32
		active, confirmed                     bool
	)
	err := db.QueryRow(`
		SELECT "alias", "name", "surname", "email", "password", "role", "active", "confirmed"
		FROM "profile" WHERE "id" = $1
	`, profile.ID).Scan(&alias, &name, &surname, &email, &password, &role, &active, &confirmed)
	require.NoError(t, err)

	assert.Equal(t, profile.Alias, alias)
	assert.Equal(t, profile.Role, role)
	assert.Equal(t, profile.Active, active)
	assert.Equal(t, profile.Confirmed, confirmed)

	custom := b.Profile(factory.ProfileParams{
		Alias:     "custom01",
		Name:      "Jane",
		Surname:   "Doe",
		Email:     "jane@example.com",
		Password:  "secret",
		Role:      5,
		Active:    factory.Bool(false),
		Confirmed: factory.Bool(false),
	})

	assert.True(t, uuid.IsValid(custom.ID), "custom profile ID must be a valid UUID")
	assert.True(t, crypto.CheckPasswordHash("secret", custom.Password), "custom password hash must verify")
	assert.Equal(t, "custom01", custom.Alias)
	assert.Equal(t, int32(5), custom.Role)
	assert.False(t, custom.Active)
	assert.False(t, custom.Confirmed)

	err = db.QueryRow(`
		SELECT "name", "surname", "email" FROM "profile" WHERE "id" = $1
	`, custom.ID).Scan(&name, &surname, &email)
	require.NoError(t, err)

	assert.Equal(t, "Jane", name)
	assert.Equal(t, "Doe", surname)
	assert.Equal(t, "jane@example.com", email)
}

func TestBuilderProject(t *testing.T) {
	b, db := newBuilder(t)

	owner := b.Profile(factory.ProfileParams{})
	project := b.Project(factory.ProjectParams{OwnerID: owner.ID})

	assert.True(t, uuid.IsValid(project.ID), "project ID must be a valid UUID")
	assert.Equal(t, "project", project.Title, "default title must be project")
	assert.Len(t, project.Alias, 8, "default alias must have 8 characters")

	var title, alias, ownerID string
	err := db.QueryRow(`
		SELECT "title", "alias", "owner_id" FROM "project" WHERE "id" = $1
	`, project.ID).Scan(&title, &alias, &ownerID)
	require.NoError(t, err)

	assert.Equal(t, "project", title)
	assert.Equal(t, project.Alias, alias)
	assert.Equal(t, owner.ID, ownerID)

	customOwner := b.Profile(factory.ProfileParams{})
	custom := b.Project(factory.ProjectParams{
		OwnerID: customOwner.ID,
		Title:   "custom project",
		Alias:   "custom02",
	})

	assert.Equal(t, "custom project", custom.Title)
	assert.Equal(t, "custom02", custom.Alias)
	assert.Equal(t, customOwner.ID, custom.OwnerID)
}

func TestBuilderProjectMember(t *testing.T) {
	b, db := newBuilder(t)

	profile := b.Profile(factory.ProfileParams{})
	project := b.Project(factory.ProjectParams{OwnerID: profile.ID})
	member := b.ProjectMember(factory.ProjectMemberParams{
		ProjectID: project.ID,
		ProfileID: profile.ID,
	})

	assert.True(t, uuid.IsValid(member.ID), "member ID must be a valid UUID")
	assert.Equal(t, int16(1), member.Role, "default role must be 1")
	assert.True(t, member.Active, "member must be active by default")

	var (
		projectID, profileID string
		role                 int16
		active, online       bool
	)
	err := db.QueryRow(`
		SELECT "project_id", "profile_id", "role", "active", "online"
		FROM "project_member" WHERE "id" = $1
	`, member.ID).Scan(&projectID, &profileID, &role, &active, &online)
	require.NoError(t, err)

	assert.Equal(t, project.ID, projectID)
	assert.Equal(t, profile.ID, profileID)
	assert.Equal(t, int16(1), role)
	assert.True(t, active)
	assert.False(t, online)
}

func TestBuilderProjectAPI(t *testing.T) {
	b, db := newBuilder(t)

	profile := b.Profile(factory.ProfileParams{})
	project := b.Project(factory.ProjectParams{OwnerID: profile.ID})
	api := b.ProjectAPI(factory.ProjectAPIParams{ProjectID: project.ID})

	assert.True(t, uuid.IsValid(api.ID), "API ID must be a valid UUID")
	assert.Len(t, api.APIKey, 37, "API key must have 37 characters")
	assert.Len(t, api.APISecret, 37, "API secret must have 37 characters")

	alphaNum := regexp.MustCompile(`^[a-zA-Z0-9]{37}$`)
	assert.Regexp(t, alphaNum, api.APIKey)
	assert.Regexp(t, alphaNum, api.APISecret)
	assert.True(t, api.Active, "API must be active by default")

	var apiKey, apiSecret string
	var active bool
	err := db.QueryRow(`
		SELECT "api_key", "api_secret", "active" FROM "project_api" WHERE "id" = $1
	`, api.ID).Scan(&apiKey, &apiSecret, &active)
	require.NoError(t, err)

	assert.Equal(t, api.APIKey, apiKey)
	assert.Equal(t, api.APISecret, apiSecret)
	assert.True(t, active)
}

func TestBuilderProjectLDAP(t *testing.T) {
	b, db := newBuilder(t)

	profile := b.Profile(factory.ProfileParams{})
	project := b.Project(factory.ProjectParams{OwnerID: profile.ID})
	ldap := b.ProjectLDAP(factory.ProjectLDAPParams{
		ProjectID: project.ID,
		Host:      "ldap.example.com",
		Port:      389,
	})

	assert.True(t, uuid.IsValid(ldap.ID), "LDAP ID must be a valid UUID")
	assert.Equal(t, project.ID, ldap.ProjectID)

	var host string
	var port int32
	err := db.QueryRow(`
		SELECT "host", "port" FROM "project_ldap" WHERE "id" = $1
	`, ldap.ID).Scan(&host, &port)
	require.NoError(t, err)

	assert.Equal(t, "ldap.example.com", host)
	assert.Equal(t, int32(389), port)
}
