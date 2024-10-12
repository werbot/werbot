package license

import (
	"testing"

	"github.com/werbot/werbot/api/auth"
	"github.com/werbot/werbot/internal/utils/test"
)

const (
	pathLicense     = "/v1/license"
	pathLicenseInfo = pathLicense + "/info"
)

func setupTest(t *testing.T) (*test.APIHandler, func(t *testing.T), map[string]string, map[string]string) {
	app, teardownTestCase := test.API(t)
	auth.New(app.Handler).Routes()
	New(app.Handler, "../../fixtures/licenses/publicKey_ok.key").Routes()
	app.AddRoute404()

	adminHeader, userHeader := app.TestProfileAuth()

	return app, teardownTestCase, adminHeader, userHeader
}
