package token

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/api/auth"
	profileenum "github.com/werbot/werbot/internal/core/profile/proto/enum"
	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
)

const (
	pathTokens         = "/v1/tokens"
	pathTokensProfile  = pathTokens + "/profile"
	pathTokensProject  = pathTokens + "/project"
	pathTokensScheme   = pathTokens + "/scheme"
	pathTokensRegister = pathTokens + "/register"
	pathTokensAccept   = pathTokens + "/accept"

	testPassword = "password123"

	adminEmail = "admin@werbot.net"
	userEmail  = "user@werbot.net"
)

// tokenEnv holds the fixture-free profile, project, scheme and token graph
// created by setupTest.
type tokenEnv struct {
	admin, user             factory.Profile
	adminHeader, userHeader map[string]string

	adminProject, userProject factory.Project
	adminScheme               factory.Scheme

	expiredInvite factory.Token
}

func setupTest(t *testing.T) (*test.APIHandler, *tokenEnv, func(t *testing.T)) {
	app, teardownTestCase := test.API(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	builder := factory.New(t, app.Postgres.Conn().Conn)

	env := &tokenEnv{}
	env.admin = builder.Profile(factory.ProfileParams{
		Alias:    "admin",
		Name:     "Penny",
		Surname:  "Hoyle",
		Email:    adminEmail,
		Password: testPassword,
		Role:     int32(profileenum.Role_admin),
	})
	env.user = builder.Profile(factory.ProfileParams{
		Alias:    "user",
		Name:     "Carly",
		Surname:  "Bender",
		Email:    userEmail,
		Password: testPassword,
		Role:     int32(profileenum.Role_user),
	})

	env.adminProject = builder.Project(factory.ProjectParams{
		OwnerID: env.admin.ID,
		Title:   "project1",
	})
	env.userProject = builder.Project(factory.ProjectParams{
		OwnerID: env.user.ID,
		Title:   "project3",
	})
	env.adminScheme = builder.Scheme(factory.SchemeParams{
		ProjectID: env.adminProject.ID,
		Title:     "admin scheme",
	})

	// Expired registration invite used by the negative register case.
	env.expiredInvite = builder.Token(factory.TokenParams{
		Section: int16(tokenenum.Section_profile),
		Action:  int16(tokenenum.Action_register),
		Status:  int16(tokenenum.Status_sent),
		Data: map[string]any{
			"name":    "Expired",
			"surname": "Invite",
			"email":   "expired.invite@werbot.net",
		},
		ExpiredAt: factory.Time(time.Now().Add(-time.Hour)),
	})

	env.adminHeader = app.AuthHeader(env.admin.Email, testPassword)
	env.userHeader = app.AuthHeader(env.user.Email, testPassword)

	return app, env, teardownTestCase
}

// apiRequest performs a request and returns the HTTP status and decoded body.
func apiRequest(t *testing.T, app *test.APIHandler, method, path string, headers map[string]string, body any) (int, map[string]any) {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		require.NoError(t, err)
	}

	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := app.App.Test(request, -1)
	require.NoError(t, err)
	defer response.Body.Close()

	decoded := map[string]any{}
	_ = json.NewDecoder(response.Body).Decode(&decoded)

	return response.StatusCode, decoded
}

// responseResult extracts the result object from a successful response.
func responseResult(t *testing.T, body map[string]any) map[string]any {
	t.Helper()

	result, ok := body["result"].(map[string]any)
	require.True(t, ok, "response has no result object: %#v", body)
	return result
}

func responseToken(t *testing.T, body map[string]any) string {
	t.Helper()

	value, ok := responseResult(t, body)["token"].(string)
	require.True(t, ok, "response has no token: %#v", body)
	require.NotEmpty(t, value)
	return value
}

func tokenStatus(t *testing.T, app *test.APIHandler, token string) int {
	t.Helper()

	var status int
	err := app.Postgres.Conn().Conn.QueryRow(`SELECT "status" FROM "token" WHERE "id" = $1`, token).Scan(&status)
	require.NoError(t, err)
	return status
}

func TestRegisterByInvite(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	email := "reg.invite@werbot.net"

	statusCode, body := apiRequest(t, app, http.MethodPost, pathTokensProfile, env.adminHeader, map[string]any{
		"name":    "Reg",
		"surname": "Invite",
		"email":   email,
	})
	require.Equal(t, http.StatusOK, statusCode, body)
	token := responseToken(t, body)

	statusCode, body = apiRequest(t, app, http.MethodPost, pathTokensRegister, nil, map[string]any{
		"token":    token,
		"password": testPassword,
	})
	require.Equal(t, http.StatusOK, statusCode, body)

	result := responseResult(t, body)
	profileID, ok := result["profile_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, profileID)

	var alias string
	var active, confirmed bool
	var storedEmail string
	err := app.Postgres.Conn().Conn.QueryRow(`
    SELECT "alias", "email", "active", "confirmed"
    FROM "profile"
    WHERE "id" = $1
  `, profileID).Scan(&alias, &storedEmail, &active, &confirmed)
	require.NoError(t, err)
	require.Equal(t, "reginvite", alias)
	require.Equal(t, email, storedEmail)
	require.True(t, active)
	require.True(t, confirmed)

	require.Equal(t, int(tokenenum.Status_done), tokenStatus(t, app, token))

	// A used token cannot register a second profile.
	statusCode, body = apiRequest(t, app, http.MethodPost, pathTokensRegister, nil, map[string]any{
		"token":    token,
		"password": testPassword,
	})
	require.Equal(t, http.StatusBadRequest, statusCode, body)

	// An expired token is rejected too.
	statusCode, body = apiRequest(t, app, http.MethodPost, pathTokensRegister, nil, map[string]any{
		"token":    env.expiredInvite.ID,
		"password": testPassword,
	})
	require.Equal(t, http.StatusBadRequest, statusCode, body)
}

func TestAcceptProjectInvite(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	email := "project.invite@werbot.net"

	statusCode, body := apiRequest(t, app, http.MethodPost, test.PathGluing(pathTokensProject, env.adminProject.ID), env.adminHeader, map[string]any{
		"email":   email,
		"name":    "Project",
		"surname": "Invite",
	})
	require.Equal(t, http.StatusOK, statusCode, body)
	token := responseToken(t, body)

	statusCode, body = apiRequest(t, app, http.MethodPost, pathTokensAccept, nil, map[string]any{
		"token":    token,
		"password": testPassword,
	})
	require.Equal(t, http.StatusOK, statusCode, body)

	result := responseResult(t, body)
	profileID, ok := result["profile_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, profileID)
	require.Equal(t, float64(tokenenum.Section_project), result["section"])

	var memberID string
	err := app.Postgres.Conn().Conn.QueryRow(`
    SELECT "id"
    FROM "project_member"
    WHERE "project_id" = $1 AND "profile_id" = $2 AND "active" = true AND "role" = $3
  `, env.adminProject.ID, profileID, int16(profileenum.Role_user)).Scan(&memberID)
	require.NoError(t, err)

	require.Equal(t, int(tokenenum.Status_done), tokenStatus(t, app, token))

	// The invite cannot be accepted twice.
	statusCode, body = apiRequest(t, app, http.MethodPost, pathTokensAccept, nil, map[string]any{
		"token":    token,
		"password": testPassword,
	})
	require.Equal(t, http.StatusBadRequest, statusCode, body)
}

func TestAcceptSchemeInvite(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	email := "scheme.invite@werbot.net"

	statusCode, body := apiRequest(t, app, http.MethodPost, test.PathGluing(pathTokensScheme, env.adminScheme.ID), env.adminHeader, map[string]any{
		"email": email,
	})
	require.Equal(t, http.StatusOK, statusCode, body)
	token := responseToken(t, body)

	// Scheme invites carry only an email, so the profile data is supplied here.
	statusCode, body = apiRequest(t, app, http.MethodPost, pathTokensAccept, nil, map[string]any{
		"token":    token,
		"password": testPassword,
		"name":     "Scheme",
		"surname":  "Invite",
	})
	require.Equal(t, http.StatusOK, statusCode, body)

	result := responseResult(t, body)
	profileID, ok := result["profile_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, profileID)
	require.Equal(t, float64(tokenenum.Section_scheme), result["section"])

	var projectMemberID string
	err := app.Postgres.Conn().Conn.QueryRow(`
    SELECT "project_member"."id"
    FROM "project_member"
    WHERE "project_member"."project_id" = $1 AND "project_member"."profile_id" = $2
  `, env.adminProject.ID, profileID).Scan(&projectMemberID)
	require.NoError(t, err)

	var schemeMemberID string
	err = app.Postgres.Conn().Conn.QueryRow(`
    SELECT "id"
    FROM "scheme_member"
    WHERE "scheme_id" = $1 AND "project_member_id" = $2 AND "active" = true
  `, env.adminScheme.ID, projectMemberID).Scan(&schemeMemberID)
	require.NoError(t, err)

	require.Equal(t, int(tokenenum.Status_done), tokenStatus(t, app, token))
}

func TestProjectTokensPrivacyAndDelete(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	// The user owns the invite they create. The project list starts empty.
	statusCode, body := apiRequest(t, app, http.MethodPost, test.PathGluing(pathTokensProject, env.userProject.ID), env.userHeader, map[string]any{
		"email":   "privacy.user@werbot.net",
		"name":    "Privacy",
		"surname": "User",
	})
	require.Equal(t, http.StatusOK, statusCode, body)
	userToken := responseToken(t, body)

	statusCode, body = apiRequest(t, app, http.MethodGet, test.PathGluing(pathTokensProject, env.userProject.ID), env.userHeader, nil)
	require.Equal(t, http.StatusOK, statusCode, body)
	require.Equal(t, float64(1), responseResult(t, body)["total"])

	// A non-owner does not see another profile's invites.
	statusCode, body = apiRequest(t, app, http.MethodGet, test.PathGluing(pathTokensProject, env.adminProject.ID), env.userHeader, nil)
	require.Equal(t, http.StatusNotFound, statusCode, body)

	// A non-owner cannot delete another profile's invite.
	statusCode, body = apiRequest(t, app, http.MethodPost, test.PathGluing(pathTokensProject, env.adminProject.ID), env.adminHeader, map[string]any{
		"email":   "privacy.admin@werbot.net",
		"name":    "Privacy",
		"surname": "Admin",
	})
	require.Equal(t, http.StatusOK, statusCode, body)
	adminToken := responseToken(t, body)

	statusCode, body = apiRequest(t, app, http.MethodDelete, test.PathGluing(pathTokens, adminToken), env.userHeader, nil)
	require.Equal(t, http.StatusNotFound, statusCode, body)

	// The owner deletes their own invite.
	statusCode, body = apiRequest(t, app, http.MethodDelete, test.PathGluing(pathTokens, userToken), env.userHeader, nil)
	require.Equal(t, http.StatusOK, statusCode, body)
	require.Equal(t, int(tokenenum.Status_deleted), tokenStatus(t, app, userToken))

	// An admin may delete a token owned by another profile.
	statusCode, body = apiRequest(t, app, http.MethodPost, test.PathGluing(pathTokensProject, env.userProject.ID), env.userHeader, map[string]any{
		"email":   "privacy.second@werbot.net",
		"name":    "Privacy",
		"surname": "Second",
	})
	require.Equal(t, http.StatusOK, statusCode, body)
	secondUserToken := responseToken(t, body)

	statusCode, body = apiRequest(t, app, http.MethodDelete, test.PathGluing(pathTokens, secondUserToken), env.adminHeader, nil)
	require.Equal(t, http.StatusOK, statusCode, body)
	require.Equal(t, int(tokenenum.Status_deleted), tokenStatus(t, app, secondUserToken))
}
