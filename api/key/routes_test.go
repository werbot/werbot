package key

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/api/auth"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
)

const (
	pathKeys         = "/v1/keys"
	pathKeysGenerate = pathKeys + "/generate"

	adminPassword = "admin@werbot.net"
	userPassword  = "user@werbot.net"
)

type keyEnv struct {
	admin, user             factory.Profile
	adminKey                factory.PublicKey
	lockedUserKey           factory.PublicKey
	unlockedUserKey         factory.PublicKey
	adminHeader, userHeader map[string]string
}

func setupTest(t *testing.T) (*test.APIHandler, *keyEnv, func(t *testing.T)) {
	app, teardownTestCase := test.API(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	builder := factory.New(t, app.Postgres.Conn().Conn)

	env := &keyEnv{}
	env.admin = builder.Profile(factory.ProfileParams{
		Alias:    "admin",
		Email:    adminPassword,
		Password: adminPassword,
		Role:     3,
	})
	env.user = builder.Profile(factory.ProfileParams{
		Alias:    "user",
		Email:    userPassword,
		Password: userPassword,
		Role:     1,
	})

	env.adminKey = builder.PublicKey(factory.PublicKeyParams{
		ProfileID: env.admin.ID,
		Title:     "public_key 1",
	})
	env.unlockedUserKey = builder.PublicKey(factory.PublicKeyParams{
		ProfileID: env.user.ID,
		Title:     "public_key 5",
	})
	env.lockedUserKey = builder.PublicKey(factory.PublicKeyParams{
		ProfileID: env.user.ID,
		Title:     "public_key 4",
		LockedAt:  factory.Time(time.Now()),
		UpdatedAt: factory.Time(time.Now()),
	})

	env.adminHeader = app.AuthHeader(env.admin.Email, adminPassword)
	env.userHeader = app.AuthHeader(env.user.Email, userPassword)

	return app, env, teardownTestCase
}

type keyListItem struct {
	KeyID       string  `json:"key_id"`
	ProfileID   string  `json:"profile_id"`
	Title       string  `json:"title"`
	Key         *string `json:"key"`
	Fingerprint string  `json:"fingerprint"`
	LockedAt    *string `json:"locked_at"`
	ArchivedAt  *string `json:"archived_at"`
	UpdatedAt   *string `json:"updated_at"`
}

// findKeyInList fetches a key list and returns the key with keyID. The list
// query has no ORDER BY, so the test must not depend on row indexes.
func findKeyInList(t *testing.T, app *test.APIHandler, path string, headers map[string]string, keyID string) keyListItem {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		request.Header.Add(k, v)
	}

	response, err := app.App.Test(request, -1)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)

	var body struct {
		Result struct {
			PublicKeys []keyListItem `json:"public_keys"`
		} `json:"result"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&body))

	for _, key := range body.Result.PublicKeys {
		if key.KeyID == keyID {
			return key
		}
	}

	require.Failf(t, "key not found", "key %s is missing from %s", keyID, path)
	return keyListItem{}
}
