package token

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
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
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	// Registration invite: the list and the public lookup expose profile metadata.
	statusCode, body := apiRequest(t, app, http.MethodPost, pathTokensProfile, env.adminHeader, map[string]any{
		"name":    "List",
		"surname": "User",
		"email":   "list.user@werbot.net",
	})
	require.Equal(t, http.StatusOK, statusCode, body)
	profileToken := responseToken(t, body)

	statusCode, body = apiRequest(t, app, http.MethodGet, pathTokensProfile, env.adminHeader, nil)
	require.Equal(t, http.StatusOK, statusCode, body)
	profileItem := findToken(t, body, profileToken)
	profileData, ok := profileItem["profile"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "list.user@werbot.net", profileData["email"])

	statusCode, body = apiRequest(t, app, http.MethodGet, pathTokens+"/"+profileToken, nil, nil)
	require.Equal(t, http.StatusOK, statusCode, body)
	result := responseResult(t, body)
	require.Equal(t, float64(tokenenum.Section_profile), result["section"])
	require.Equal(t, float64(tokenenum.Action_register), result["action"])
	profileData, ok = result["profile"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "list.user@werbot.net", profileData["email"])

	// Scheme invite: scheme tokens store profile metadata too.
	statusCode, body = apiRequest(t, app, http.MethodPost, pathTokensScheme+"/"+env.adminScheme.ID, env.adminHeader, map[string]any{
		"email": "scheme.list@werbot.net",
	})
	require.Equal(t, http.StatusOK, statusCode, body)
	schemeToken := responseToken(t, body)

	statusCode, body = apiRequest(t, app, http.MethodGet, pathTokens+"/"+schemeToken, nil, nil)
	require.Equal(t, http.StatusOK, statusCode, body)
	result = responseResult(t, body)
	require.Equal(t, float64(tokenenum.Section_scheme), result["section"])
	require.Equal(t, float64(tokenenum.Action_request), result["action"])
	schemeData, ok := result["scheme"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "scheme.list@werbot.net", schemeData["email"])

	statusCode, body = apiRequest(t, app, http.MethodGet, pathTokensScheme+"/"+env.adminScheme.ID, env.adminHeader, nil)
	require.Equal(t, http.StatusOK, statusCode, body)
	schemeItem := findToken(t, body, schemeToken)
	schemeData, ok = schemeItem["scheme"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "scheme.list@werbot.net", schemeData["email"])

	// Unknown tokens are rejected.
	statusCode, body = apiRequest(t, app, http.MethodGet, pathTokens+"/"+test.ConstFakeID, nil, nil)
	require.Equal(t, http.StatusNotFound, statusCode, body)
}
