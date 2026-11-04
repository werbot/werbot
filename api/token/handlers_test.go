package token

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/utils/test"
)

func findToken(t *testing.T, body map[string]any, token string) map[string]any {
	t.Helper()

	result := responseResult(t, body)
	rawTokens, ok := result["tokens"].([]any)
	require.True(t, ok, "response has no token list: %#v", result)

	for _, raw := range rawTokens {
		item, ok := raw.(map[string]any)
		require.True(t, ok)
		if item["token"] == token {
			return item
		}
	}

	t.Fatalf("token %s not found in list", token)
	return nil
}

func TestTokenLookupAndLists(t *testing.T) {
	app, teardownTestCase, adminHeader, _ := setupTest(t)
	defer teardownTestCase(t)

	// Registration invite: the list and the public lookup expose profile metadata.
	statusCode, body := apiRequest(t, app, http.MethodPost, pathTokensProfile, adminHeader, map[string]any{
		"name":    "List",
		"surname": "User",
		"email":   "list.user@werbot.net",
	})
	require.Equal(t, http.StatusOK, statusCode, body)
	profileToken := responseToken(t, body)

	statusCode, body = apiRequest(t, app, http.MethodGet, pathTokensProfile, adminHeader, nil)
	require.Equal(t, http.StatusOK, statusCode, body)
	profileItem := findToken(t, body, profileToken)
	profileData, ok := profileItem["profile"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "list.user@werbot.net", profileData["email"])

	statusCode, body = apiRequest(t, app, http.MethodGet, pathTokens+"/"+profileToken, nil, nil)
	require.Equal(t, http.StatusOK, statusCode, body)
	result := responseResult(t, body)
	require.Equal(t, float64(1), result["section"]) // profile
	require.Equal(t, float64(2), result["action"])  // register
	profileData, ok = result["profile"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "list.user@werbot.net", profileData["email"])

	// Scheme invite: scheme tokens store profile metadata too.
	statusCode, body = apiRequest(t, app, http.MethodPost, pathTokensScheme+"/"+testAdminSchemeID, adminHeader, map[string]any{
		"email": "scheme.list@werbot.net",
	})
	require.Equal(t, http.StatusOK, statusCode, body)
	schemeToken := responseToken(t, body)

	statusCode, body = apiRequest(t, app, http.MethodGet, pathTokens+"/"+schemeToken, nil, nil)
	require.Equal(t, http.StatusOK, statusCode, body)
	result = responseResult(t, body)
	require.Equal(t, float64(3), result["section"]) // scheme
	require.Equal(t, float64(7), result["action"])  // request
	schemeData, ok := result["scheme"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "scheme.list@werbot.net", schemeData["email"])

	statusCode, body = apiRequest(t, app, http.MethodGet, pathTokensScheme+"/"+testAdminSchemeID, adminHeader, nil)
	require.Equal(t, http.StatusOK, statusCode, body)
	schemeItem := findToken(t, body, schemeToken)
	schemeData, ok = schemeItem["scheme"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "scheme.list@werbot.net", schemeData["email"])

	// Unknown tokens are rejected.
	statusCode, body = apiRequest(t, app, http.MethodGet, pathTokens+"/"+test.ConstFakeID, nil, nil)
	require.Equal(t, http.StatusNotFound, statusCode, body)
}
