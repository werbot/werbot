package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/utils/test"
)

func authRequest(t *testing.T, app *test.APIHandler, method, path string, body any) (int, map[string]any) {
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

	response, err := app.App.Test(request, -1)
	require.NoError(t, err)
	defer response.Body.Close()

	decoded := map[string]any{}
	_ = json.NewDecoder(response.Body).Decode(&decoded)

	return response.StatusCode, decoded
}

func TestHandler_passwordReset(t *testing.T) {
	app, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	// Reject an invalid email.
	statusCode, body := authRequest(t, app, http.MethodPost, pathAuthPasswordReset, map[string]any{
		"email": "not-an-email",
	})
	require.Equal(t, http.StatusBadRequest, statusCode, body)

	// Request a reset token for the admin profile.
	statusCode, body = authRequest(t, app, http.MethodPost, pathAuthPasswordReset, map[string]any{
		"email": test.ConstAdminEmail,
	})
	require.Equal(t, http.StatusOK, statusCode, body)

	var token string
	err := app.Postgres.Conn().Conn.QueryRow(`
    SELECT "id"
    FROM "token"
    WHERE "profile_id" = $1 AND "section" = 1 AND "action" = 1 AND "status" = 2
    ORDER BY "created_at" DESC
    LIMIT 1
  `, test.ConstAdminID).Scan(&token)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Check the token and reset the password.
	statusCode, body = authRequest(t, app, http.MethodGet, pathAuthPasswordReset+"/"+token, nil)
	require.Equal(t, http.StatusOK, statusCode, body)

	const newPassword = "newpassword123"
	statusCode, body = authRequest(t, app, http.MethodPost, pathAuthPasswordReset, map[string]any{
		"password": map[string]any{
			"password": newPassword,
			"token":    token,
		},
	})
	require.Equal(t, http.StatusOK, statusCode, body)

	var tokenStatus int
	err = app.Postgres.Conn().Conn.QueryRow(`SELECT "status" FROM "token" WHERE "id" = $1`, token).Scan(&tokenStatus)
	require.NoError(t, err)
	require.Equal(t, 3, tokenStatus) // used

	// The new password can be used to sign in.
	statusCode, body = authRequest(t, app, http.MethodPost, pathAuthSignIn, map[string]any{
		"email":    test.ConstAdminEmail,
		"password": newPassword,
	})
	require.Equal(t, http.StatusOK, statusCode, body)
}
