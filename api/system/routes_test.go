package system

import (
	"testing"

	"github.com/werbot/werbot/api/auth"
	"github.com/werbot/werbot/internal/utils/test"
)

const (
	pathSystem          = "/v1/system"
	pathSystemInfo      = pathSystem + "/info"
	pathSystemUpdate    = pathSystem + "/update"
	pathSystemVersion   = pathSystem + "/version"
	pathSystemMyIP      = pathSystem + "/myip"
	pathSystemCountries = pathSystem + "/countries"
)

func setupTest(t *testing.T) (*test.APIHandler, func(t *testing.T), map[string]string, map[string]string) {
	app, teardownTestCase := test.API(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	adminHeader, userHeader := app.TestUserAuth()

	return app, teardownTestCase, adminHeader, userHeader
}
