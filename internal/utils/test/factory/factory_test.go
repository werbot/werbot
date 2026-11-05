package factory_test

import (
	"database/sql"
	"encoding/json"
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

func TestBuilderScheme(t *testing.T) {
	b, db := newBuilder(t)

	profile := b.Profile(factory.ProfileParams{})
	project := b.Project(factory.ProjectParams{OwnerID: profile.ID})

	access := map[string]any{
		"alias":   "scheme01",
		"address": "127.0.0.1",
		"port":    22,
		"password": map[string]any{
			"login":    "user",
			"password": "password",
		},
	}
	scheme := b.Scheme(factory.SchemeParams{
		ProjectID:    project.ID,
		Title:        "custom scheme",
		Description:  "custom description",
		SchemeType:   103,
		Access:       access,
		AccessPolicy: map[string]any{"country": 1, "network": 1},
		Active:       factory.Bool(false),
		Audit:        factory.Bool(true),
		Online:       factory.Bool(true),
	})

	assert.True(t, uuid.IsValid(scheme.ID), "scheme ID must be a valid UUID")
	assert.Equal(t, project.ID, scheme.ProjectID)
	assert.Equal(t, "custom scheme", scheme.Title)
	assert.Equal(t, int16(103), scheme.SchemeType)
	assert.Equal(t, access, scheme.Access)

	var (
		title, description, accessJSON, policyJSON string
		schemeType                                 int16
		active, audit, online                      bool
	)
	err := db.QueryRow(`
		SELECT "title", "description", "scheme_type", "active", "audit", "online",
			"access"::text, "access_policy"::text
		FROM "scheme" WHERE "id" = $1
	`, scheme.ID).Scan(&title, &description, &schemeType, &active, &audit, &online, &accessJSON, &policyJSON)
	require.NoError(t, err)

	assert.Equal(t, "custom scheme", title)
	assert.Equal(t, "custom description", description)
	assert.Equal(t, int16(103), schemeType)
	assert.False(t, active)
	assert.True(t, audit)
	assert.True(t, online)

	expectedAccess, err := json.Marshal(access)
	require.NoError(t, err)
	assert.JSONEq(t, string(expectedAccess), accessJSON)
	assert.JSONEq(t, `{"country": 1, "network": 1}`, policyJSON)

	defaultScheme := b.Scheme(factory.SchemeParams{ProjectID: project.ID})

	assert.True(t, uuid.IsValid(defaultScheme.ID), "default scheme ID must be a valid UUID")
	assert.Equal(t, "scheme", defaultScheme.Title)
	assert.Equal(t, int16(103), defaultScheme.SchemeType)
	assert.Len(t, defaultScheme.Access["alias"], 8, "default access alias must have 8 characters")
	assert.Equal(t, "127.0.0.1", defaultScheme.Access["address"])
	assert.Equal(t, 22, defaultScheme.Access["port"])

	err = db.QueryRow(`
		SELECT "active", "audit", "online", "access_policy"::text
		FROM "scheme" WHERE "id" = $1
	`, defaultScheme.ID).Scan(&active, &audit, &online, &policyJSON)
	require.NoError(t, err)

	assert.True(t, active)
	assert.False(t, audit)
	assert.False(t, online)
	assert.JSONEq(t, `{"country": 0, "network": 0}`, policyJSON)
}

func TestBuilderSchemeMember(t *testing.T) {
	b, db := newBuilder(t)

	profile := b.Profile(factory.ProfileParams{})
	project := b.Project(factory.ProjectParams{OwnerID: profile.ID})
	projectMember := b.ProjectMember(factory.ProjectMemberParams{
		ProjectID: project.ID,
		ProfileID: profile.ID,
	})
	scheme := b.Scheme(factory.SchemeParams{ProjectID: project.ID})

	member := b.SchemeMember(factory.SchemeMemberParams{
		SchemeID:        scheme.ID,
		ProjectMemberID: projectMember.ID,
		Active:          factory.Bool(false),
		Online:          factory.Bool(true),
	})

	assert.True(t, uuid.IsValid(member.ID), "scheme member ID must be a valid UUID")
	assert.Equal(t, scheme.ID, member.SchemeID)
	assert.Equal(t, projectMember.ID, member.ProjectMemberID)
	assert.False(t, member.Active)
	assert.True(t, member.Online)

	var (
		schemeID, projectMemberID string
		active, online            bool
	)
	err := db.QueryRow(`
		SELECT "scheme_id", "project_member_id", "active", "online"
		FROM "scheme_member" WHERE "id" = $1
	`, member.ID).Scan(&schemeID, &projectMemberID, &active, &online)
	require.NoError(t, err)

	assert.Equal(t, scheme.ID, schemeID)
	assert.Equal(t, projectMember.ID, projectMemberID)
	assert.False(t, active)
	assert.True(t, online)

	defaultMember := b.SchemeMember(factory.SchemeMemberParams{
		SchemeID:        scheme.ID,
		ProjectMemberID: projectMember.ID,
	})

	assert.True(t, defaultMember.Active, "scheme member must be active by default")
	assert.False(t, defaultMember.Online, "scheme member must be offline by default")
}

func TestBuilderSchemeActivity(t *testing.T) {
	b, db := newBuilder(t)

	profile := b.Profile(factory.ProfileParams{})
	project := b.Project(factory.ProjectParams{OwnerID: profile.ID})
	scheme := b.Scheme(factory.SchemeParams{ProjectID: project.ID})

	week := map[string][]int32{}
	for _, day := range []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"} {
		hours := make([]int32, 24)
		for i := range hours {
			hours[i] = int32(i % 2)
		}
		week[day] = hours
	}

	b.SchemeActivity(scheme.ID, week)

	var (
		count     int
		dataJSON  string
		updatedAt sql.NullTime
	)
	err := db.QueryRow(`
		SELECT COUNT(*), MIN("data"::text), MIN("updated_at")
		FROM "scheme_activity" WHERE "scheme_id" = $1
	`, scheme.ID).Scan(&count, &dataJSON, &updatedAt)
	require.NoError(t, err)

	assert.Equal(t, 1, count, "trigger-created activity row must be updated, not duplicated")

	var got map[string][]int32
	require.NoError(t, json.Unmarshal([]byte(dataJSON), &got))
	assert.Equal(t, week, got)
}

func TestBuilderSchemeHostKey(t *testing.T) {
	b, db := newBuilder(t)

	profile := b.Profile(factory.ProfileParams{})
	project := b.Project(factory.ProjectParams{OwnerID: profile.ID})
	scheme := b.Scheme(factory.SchemeParams{ProjectID: project.ID})

	key := []byte("ssh-rsa host key")
	b.SchemeHostKey(scheme.ID, key)

	var (
		count   int
		hostKey []byte
	)
	err := db.QueryRow(`
		SELECT COUNT(*), MIN("host_key")
		FROM "scheme_host_key" WHERE "scheme_id" = $1
	`, scheme.ID).Scan(&count, &hostKey)
	require.NoError(t, err)

	assert.Equal(t, 1, count, "trigger-created host key row must be updated, not duplicated")
	assert.Equal(t, key, hostKey)

	noTrigger := b.Scheme(factory.SchemeParams{ProjectID: project.ID, SchemeType: 101})
	b.SchemeHostKey(noTrigger.ID, key)

	err = db.QueryRow(`
		SELECT COUNT(*), MIN("host_key")
		FROM "scheme_host_key" WHERE "scheme_id" = $1
	`, noTrigger.ID).Scan(&count, &hostKey)
	require.NoError(t, err)

	assert.Equal(t, 1, count, "host key row must be inserted when the trigger created none")
	assert.Equal(t, key, hostKey)
}

func TestBuilderSchemeFirewallCountry(t *testing.T) {
	b, db := newBuilder(t)

	profile := b.Profile(factory.ProfileParams{})
	project := b.Project(factory.ProjectParams{OwnerID: profile.ID})
	scheme := b.Scheme(factory.SchemeParams{ProjectID: project.ID})

	country := b.SchemeFirewallCountry(factory.SchemeFirewallCountryParams{
		SchemeID:    scheme.ID,
		CountryCode: "US",
	})

	assert.True(t, uuid.IsValid(country.ID), "firewall country ID must be a valid UUID")
	assert.Equal(t, scheme.ID, country.SchemeID)
	assert.Equal(t, "US", country.CountryCode)

	var countryCode string
	err := db.QueryRow(`
		SELECT "country_code" FROM "scheme_firewall_country" WHERE "id" = $1
	`, country.ID).Scan(&countryCode)
	require.NoError(t, err)

	assert.Equal(t, "US", countryCode)
}

func newSchemeMember(t *testing.T, b *factory.Builder) factory.SchemeMember {
	t.Helper()

	profile := b.Profile(factory.ProfileParams{})
	project := b.Project(factory.ProjectParams{OwnerID: profile.ID})
	projectMember := b.ProjectMember(factory.ProjectMemberParams{
		ProjectID: project.ID,
		ProfileID: profile.ID,
	})
	scheme := b.Scheme(factory.SchemeParams{ProjectID: project.ID})

	return b.SchemeMember(factory.SchemeMemberParams{
		SchemeID:        scheme.ID,
		ProjectMemberID: projectMember.ID,
	})
}

func TestBuilderEvent(t *testing.T) {
	b, db := newBuilder(t)

	profile := b.Profile(factory.ProfileParams{})
	event := b.Event(factory.EventParams{
		RelatedID:    profile.ID,
		PrimeSection: 1,
		Section:      101,
		Type:         1,
	})

	assert.True(t, uuid.IsValid(event.ID), "event ID must be a valid UUID")
	assert.Equal(t, profile.ID, event.RelatedID)
	assert.Equal(t, int16(1), event.PrimeSection)
	assert.Equal(t, int16(101), event.Section)
	assert.Equal(t, int16(1), event.Type)

	var sessionJSON, dataJSON string
	err := db.QueryRow(`
		SELECT "session"::text, "data"::text FROM "event" WHERE "id" = $1
	`, event.ID).Scan(&sessionJSON, &dataJSON)
	require.NoError(t, err)

	var session map[string]any
	require.NoError(t, json.Unmarshal([]byte(sessionJSON), &session))

	sessionID, ok := session["id"].(string)
	require.True(t, ok, "session must contain an id")
	assert.True(t, uuid.IsValid(sessionID), "session ID must be a valid UUID")
	assert.Equal(t, "test-agent", session["user_agent"])
	assert.Equal(t, "192.0.2.1", session["ip"])
	assert.JSONEq(t, `{}`, dataJSON)
}

func TestBuilderEventSession(t *testing.T) {
	b, db := newBuilder(t)

	profile := b.Profile(factory.ProfileParams{})
	explicitID := uuid.New()
	event := b.Event(factory.EventParams{
		RelatedID:    profile.ID,
		PrimeSection: 1,
		Section:      101,
		Type:         2,
		Session: map[string]any{
			"id":         explicitID,
			"user_agent": "explicit-agent",
			"ip":         "198.51.100.7",
		},
		SessionID: uuid.New(),
		UserAgent: "ignored-agent",
		IP:        "192.0.2.99",
	})

	var sessionJSON string
	err := db.QueryRow(`SELECT "session"::text FROM "event" WHERE "id" = $1`, event.ID).Scan(&sessionJSON)
	require.NoError(t, err)

	var session map[string]any
	require.NoError(t, json.Unmarshal([]byte(sessionJSON), &session))

	assert.Equal(t, explicitID, session["id"])
	assert.Equal(t, "explicit-agent", session["user_agent"])
	assert.Equal(t, "198.51.100.7", session["ip"])

	scalarID := uuid.New()
	scalar := b.Event(factory.EventParams{
		RelatedID:    profile.ID,
		PrimeSection: 1,
		Section:      101,
		Type:         3,
		SessionID:    scalarID,
		UserAgent:    "scalar-agent",
		IP:           "203.0.113.5",
	})

	err = db.QueryRow(`SELECT "session"::text FROM "event" WHERE "id" = $1`, scalar.ID).Scan(&sessionJSON)
	require.NoError(t, err)

	require.NoError(t, json.Unmarshal([]byte(sessionJSON), &session))

	assert.Equal(t, scalarID, session["id"])
	assert.Equal(t, "scalar-agent", session["user_agent"])
	assert.Equal(t, "203.0.113.5", session["ip"])
}

func TestBuilderSession(t *testing.T) {
	b, db := newBuilder(t)

	member := newSchemeMember(t, b)
	session := b.Session(factory.SessionParams{
		MemberID: member.ID,
		Status:   factory.Bool(true),
		Message:  "connected",
	})

	assert.True(t, uuid.IsValid(session.ID), "session ID must be a valid UUID")
	assert.Equal(t, member.ID, session.MemberID)

	var (
		memberID, message string
		status            bool
	)
	err := db.QueryRow(`
		SELECT "member_id", "status", "message" FROM "session" WHERE "id" = $1
	`, session.ID).Scan(&memberID, &status, &message)
	require.NoError(t, err)

	assert.Equal(t, member.ID, memberID)
	assert.True(t, status)
	assert.Equal(t, "connected", message)
}

func TestBuilderAudit(t *testing.T) {
	b, db := newBuilder(t)

	member := newSchemeMember(t, b)
	audit := b.Audit(factory.AuditParams{
		MemberID: member.ID,
		Version:  2,
		Width:    120,
		Height:   40,
		Duration: 12.5,
		Command:  "ls -la",
		Title:    "bash",
		EnvTerm:  "xterm-256color",
		EnvShell: "/bin/bash",
		Session:  "audit-session",
		ClientIP: "192.0.2.5",
	})

	assert.True(t, uuid.IsValid(audit.ID), "audit ID must be a valid UUID")
	assert.Equal(t, member.ID, audit.MemberID)

	var (
		memberID, command, title, envTerm, envShell, session, clientIP string
		version, width, height                                         int16
		duration                                                       float64
		timeStart                                                      sql.NullTime
	)
	err := db.QueryRow(`
		SELECT "member_id", "time_start", "version", "width", "height", "duration",
			"command", "title", "env_term", "env_shell", "session", host("client_ip")
		FROM "audit" WHERE "id" = $1
	`, audit.ID).Scan(&memberID, &timeStart, &version, &width, &height, &duration,
		&command, &title, &envTerm, &envShell, &session, &clientIP)
	require.NoError(t, err)

	assert.Equal(t, member.ID, memberID)
	assert.True(t, timeStart.Valid, "time_start must be set by default")
	assert.Equal(t, int16(2), version)
	assert.Equal(t, int16(120), width)
	assert.Equal(t, int16(40), height)
	assert.Equal(t, 12.5, duration)
	assert.Equal(t, "ls -la", command)
	assert.Equal(t, "bash", title)
	assert.Equal(t, "xterm-256color", envTerm)
	assert.Equal(t, "/bin/bash", envShell)
	assert.Equal(t, "audit-session", session)
	assert.Equal(t, "192.0.2.5", clientIP)
}

func TestBuilderAuditRecord(t *testing.T) {
	b, db := newBuilder(t)

	member := newSchemeMember(t, b)
	audit := b.Audit(factory.AuditParams{MemberID: member.ID})
	record := b.AuditRecord(factory.AuditRecordParams{
		AuditID:  audit.ID,
		Duration: 0.25,
		Screen:   "screen data",
		Type:     "o",
	})

	assert.True(t, uuid.IsValid(record.ID), "audit record ID must be a valid UUID")
	assert.Equal(t, audit.ID, record.AuditID)

	var (
		auditID, screen, recordType string
		duration                    float64
	)
	err := db.QueryRow(`
		SELECT "audit_id", "duration", "screen", "type"
		FROM "audit_record" WHERE "id" = $1
	`, record.ID).Scan(&auditID, &duration, &screen, &recordType)
	require.NoError(t, err)

	assert.Equal(t, audit.ID, auditID)
	assert.Equal(t, 0.25, duration)
	assert.Equal(t, "screen data", screen)
	assert.Equal(t, "o", recordType)
}

func TestBuilderToken(t *testing.T) {
	b, db := newBuilder(t)

	nullToken := b.Token(factory.TokenParams{Section: 1, Action: 2, Status: 3})

	assert.True(t, uuid.IsValid(nullToken.ID), "token ID must be a valid UUID")

	var (
		profileID, projectID, schemeID sql.NullString
		section, action, status        int16
		dataJSON                       string
	)
	err := db.QueryRow(`
		SELECT "section", "action", "status", "profile_id", "project_id", "scheme_id", "data"::text
		FROM "token" WHERE "id" = $1
	`, nullToken.ID).Scan(&section, &action, &status, &profileID, &projectID, &schemeID, &dataJSON)
	require.NoError(t, err)

	assert.Equal(t, int16(1), section)
	assert.Equal(t, int16(2), action)
	assert.Equal(t, int16(3), status)
	assert.False(t, profileID.Valid, "empty profile ID must be stored as NULL")
	assert.False(t, projectID.Valid, "empty project ID must be stored as NULL")
	assert.False(t, schemeID.Valid, "empty scheme ID must be stored as NULL")
	assert.JSONEq(t, `{}`, dataJSON)

	profile := b.Profile(factory.ProfileParams{})
	project := b.Project(factory.ProjectParams{OwnerID: profile.ID})
	scheme := b.Scheme(factory.SchemeParams{ProjectID: project.ID})
	linked := b.Token(factory.TokenParams{
		Section:   1,
		Action:    2,
		Status:    3,
		ProfileID: profile.ID,
		ProjectID: project.ID,
		SchemeID:  scheme.ID,
		Data:      map[string]any{"key": "value"},
	})

	assert.True(t, uuid.IsValid(linked.ID), "linked token ID must be a valid UUID")

	err = db.QueryRow(`
		SELECT "profile_id", "project_id", "scheme_id", "data"::text
		FROM "token" WHERE "id" = $1
	`, linked.ID).Scan(&profileID, &projectID, &schemeID, &dataJSON)
	require.NoError(t, err)

	assert.Equal(t, profile.ID, profileID.String)
	assert.Equal(t, project.ID, projectID.String)
	assert.Equal(t, scheme.ID, schemeID.String)
	assert.JSONEq(t, `{"key": "value"}`, dataJSON)
}

func TestBuilderFirewallList(t *testing.T) {
	b, db := newBuilder(t)

	listID := b.FirewallList(factory.FirewallListParams{Name: "test.netset"})

	assert.True(t, uuid.IsValid(listID), "firewall list ID must be a valid UUID")

	var (
		name   string
		active bool
		path   int16
	)
	err := db.QueryRow(`
		SELECT "name", "active", "path" FROM "firewall_list" WHERE "id" = $1
	`, listID).Scan(&name, &active, &path)
	require.NoError(t, err)

	assert.Equal(t, "test.netset", name)
	assert.True(t, active, "firewall list must be active by default")
	assert.Equal(t, int16(0), path)

	b.FirewallCountry(listID, "US")

	var countryCode string
	err = db.QueryRow(`
		SELECT "country_code" FROM "firewall_country" WHERE "firewall_list_id" = $1
	`, listID).Scan(&countryCode)
	require.NoError(t, err)

	assert.Equal(t, "US", countryCode)

	b.FirewallNetwork(listID, "10.0.0.0/8")

	var network string
	err = db.QueryRow(`
		SELECT "network"::text FROM "firewall_network" WHERE "firewall_list_id" = $1
	`, listID).Scan(&network)
	require.NoError(t, err)

	assert.Equal(t, "10.0.0.0/8", network)
}

func TestBuilderSchemeFirewallNetwork(t *testing.T) {
	b, db := newBuilder(t)

	profile := b.Profile(factory.ProfileParams{})
	project := b.Project(factory.ProjectParams{OwnerID: profile.ID})
	scheme := b.Scheme(factory.SchemeParams{ProjectID: project.ID})

	network := b.SchemeFirewallNetwork(factory.SchemeFirewallNetworkParams{
		SchemeID: scheme.ID,
		Network:  "10.0.0.0/8",
	})

	assert.True(t, uuid.IsValid(network.ID), "firewall network ID must be a valid UUID")
	assert.Equal(t, scheme.ID, network.SchemeID)
	assert.Equal(t, "10.0.0.0/8", network.Network)

	var got string
	err := db.QueryRow(`
		SELECT "network"::text FROM "scheme_firewall_network" WHERE "id" = $1
	`, network.ID).Scan(&got)
	require.NoError(t, err)

	assert.Equal(t, "10.0.0.0/8", got)
}
