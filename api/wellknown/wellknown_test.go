package wellknown

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_jwks(t *testing.T) {
	app, teardownTestCase := test.API(t)
	defer teardownTestCase(t)

	New(app.Handler).Routes()

	response, err := app.App.Test(httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil), -1)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)

	body := &JWKSResponse{}
	require.NoError(t, json.NewDecoder(response.Body).Decode(body))
	require.Len(t, body.Keys, 1)
	require.NotEmpty(t, body.Keys[0].KeyType)
	require.NotEmpty(t, body.Keys[0].N)
	require.NotEmpty(t, body.Keys[0].E)

	// A missing key file is reported as a server error.
	t.Setenv("JWT_PUBLIC_KEY", "missing-public.key")
	response, err = app.App.Test(httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil), -1)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusInternalServerError, response.StatusCode)
}
