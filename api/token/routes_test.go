package token

import (
	"testing"

	"github.com/werbot/werbot/api/auth"
	"github.com/werbot/werbot/internal/utils/test"
)

const (
	pathTokens        = "/v1/tokens"
	pathTokensProfile = pathTokens + "/profile"
	pathTokensProject = pathTokens + "/project"
	pathTokensScheme  = pathTokens + "/scheme"
)

func setupTest(t *testing.T) (*test.APIHandler, func(t *testing.T), map[string]string, map[string]string) {
	app, teardownTestCase := test.API(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	adminHeader, userHeader := app.TestProfileAuth()

	return app, teardownTestCase, adminHeader, userHeader
}
