package token

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/api/auth"
	"github.com/werbot/werbot/internal/utils/test"
)

const (
	pathTokens         = "/v1/tokens"
	pathTokensProfile  = pathTokens + "/profile"
	pathTokensProject  = pathTokens + "/project"
	pathTokensScheme   = pathTokens + "/scheme"
	pathTokensRegister = pathTokens + "/register"
	pathTokensAccept   = pathTokens + "/accept"

	testPassword      = "password123"
	testAdminSchemeID = "0c3a8869-6fc0-4666-bf60-15475473392a"
)

func setupTest(t *testing.T) (*test.APIHandler, func(t *testing.T), map[string]string, map[string]string) {
	app, teardownTestCase := test.API(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	adminHeader, userHeader := app.TestProfileAuth()

	return app, teardownTestCase, adminHeader, userHeader
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
	app, teardownTestCase, adminHeader, _ := setupTest(t)
	defer teardownTestCase(t)

	email := "reg.invite@werbot.net"

	statusCode, body := apiRequest(t, app, http.MethodPost, pathTokensProfile, adminHeader, map[string]any{
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

	require.Equal(t, 1, tokenStatus(t, app, token)) // done

	// A used token cannot register a second profile.
	statusCode, body = apiRequest(t, app, http.MethodPost, pathTokensRegister, nil, map[string]any{
		"token":    token,
		"password": testPassword,
	})
	require.Equal(t, http.StatusBadRequest, statusCode, body)

	// An expired token is rejected too.
	statusCode, body = apiRequest(t, app, http.MethodPost, pathTokensRegister, nil, map[string]any{
		"token":    "1b8bf7fe-d901-4c12-9c6c-e1689e45490f",
		"password": testPassword,
	})
	require.Equal(t, http.StatusBadRequest, statusCode, body)
}

func TestAcceptProjectInvite(t *testing.T) {
	app, teardownTestCase, adminHeader, _ := setupTest(t)
	defer teardownTestCase(t)

	email := "project.invite@werbot.net"

	statusCode, body := apiRequest(t, app, http.MethodPost, test.PathGluing(pathTokensProject, test.ConstAdminProject1ID), adminHeader, map[string]any{
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
	require.Equal(t, float64(2), result["section"]) // project

	var memberID string
	err := app.Postgres.Conn().Conn.QueryRow(`
    SELECT "id"
    FROM "project_member"
    WHERE "project_id" = $1 AND "profile_id" = $2 AND "active" = true AND "role" = 1
  `, test.ConstAdminProject1ID, profileID).Scan(&memberID)
	require.NoError(t, err)

	require.Equal(t, 1, tokenStatus(t, app, token)) // done

	// The invite cannot be accepted twice.
	statusCode, body = apiRequest(t, app, http.MethodPost, pathTokensAccept, nil, map[string]any{
		"token":    token,
		"password": testPassword,
	})
	require.Equal(t, http.StatusBadRequest, statusCode, body)
}

func TestAcceptSchemeInvite(t *testing.T) {
	app, teardownTestCase, adminHeader, _ := setupTest(t)
	defer teardownTestCase(t)

	email := "scheme.invite@werbot.net"

	statusCode, body := apiRequest(t, app, http.MethodPost, test.PathGluing(pathTokensScheme, testAdminSchemeID), adminHeader, map[string]any{
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
	require.Equal(t, float64(3), result["section"]) // scheme

	var projectMemberID string
	err := app.Postgres.Conn().Conn.QueryRow(`
    SELECT "project_member"."id"
    FROM "project_member"
    INNER JOIN "scheme" ON "scheme"."project_id" = "project_member"."project_id"
    WHERE "scheme"."id" = $1 AND "project_member"."profile_id" = $2
  `, testAdminSchemeID, profileID).Scan(&projectMemberID)
	require.NoError(t, err)

	var schemeMemberID string
	err = app.Postgres.Conn().Conn.QueryRow(`
    SELECT "id"
    FROM "scheme_member"
    WHERE "scheme_id" = $1 AND "project_member_id" = $2 AND "active" = true
  `, testAdminSchemeID, projectMemberID).Scan(&schemeMemberID)
	require.NoError(t, err)

	require.Equal(t, 1, tokenStatus(t, app, token)) // done
}

func TestProjectTokensPrivacyAndDelete(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	// The user owns the invite they create.
	statusCode, body := apiRequest(t, app, http.MethodPost, test.PathGluing(pathTokensProject, test.ConstUserProject1ID), userHeader, map[string]any{
		"email":   "privacy.user@werbot.net",
		"name":    "Privacy",
		"surname": "User",
	})
	require.Equal(t, http.StatusOK, statusCode, body)
	userToken := responseToken(t, body)

	statusCode, body = apiRequest(t, app, http.MethodGet, test.PathGluing(pathTokensProject, test.ConstUserProject1ID), userHeader, nil)
	require.Equal(t, http.StatusOK, statusCode, body)
	require.Equal(t, float64(1), responseResult(t, body)["total"])

	// A non-owner does not see another profile's invites.
	statusCode, body = apiRequest(t, app, http.MethodGet, test.PathGluing(pathTokensProject, test.ConstAdminProject1ID), userHeader, nil)
	require.Equal(t, http.StatusNotFound, statusCode, body)

	// A non-owner cannot delete another profile's invite.
	statusCode, body = apiRequest(t, app, http.MethodPost, test.PathGluing(pathTokensProject, test.ConstAdminProject1ID), adminHeader, map[string]any{
		"email":   "privacy.admin@werbot.net",
		"name":    "Privacy",
		"surname": "Admin",
	})
	require.Equal(t, http.StatusOK, statusCode, body)
	adminToken := responseToken(t, body)

	statusCode, body = apiRequest(t, app, http.MethodDelete, test.PathGluing(pathTokens, adminToken), userHeader, nil)
	require.Equal(t, http.StatusNotFound, statusCode, body)

	// The owner deletes their own invite.
	statusCode, body = apiRequest(t, app, http.MethodDelete, test.PathGluing(pathTokens, userToken), userHeader, nil)
	require.Equal(t, http.StatusOK, statusCode, body)
	require.Equal(t, 4, tokenStatus(t, app, userToken)) // deleted

	// An admin may delete a token owned by another profile.
	statusCode, body = apiRequest(t, app, http.MethodPost, test.PathGluing(pathTokensProject, test.ConstUserProject1ID), userHeader, map[string]any{
		"email":   "privacy.second@werbot.net",
		"name":    "Privacy",
		"surname": "Second",
	})
	require.Equal(t, http.StatusOK, statusCode, body)
	secondUserToken := responseToken(t, body)

	statusCode, body = apiRequest(t, app, http.MethodDelete, test.PathGluing(pathTokens, secondUserToken), adminHeader, nil)
	require.Equal(t, http.StatusOK, statusCode, body)
	require.Equal(t, 4, tokenStatus(t, app, secondUserToken)) // deleted
}
