package license

import (
	"testing"

	"github.com/werbot/werbot/api/auth"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
)

const (
	pathLicense     = "/v1/license"
	pathLicenseInfo = pathLicense + "/info"

	adminEmail    = "admin@werbot.net"
	adminPassword = "admin@werbot.net"
	userEmail     = "user@werbot.net"
	userPassword  = "user@werbot.net"
)

func setupTest(t *testing.T) (*test.APIHandler, func(t *testing.T), map[string]string, map[string]string) {
	app, teardownTestCase := test.APIWithoutFixtures(t)
	auth.New(app.Handler).Routes()
	New(app.Handler, "../../fixtures/licenses/publicKey_ok.key").Routes()
	app.AddRoute404()

	builder := factory.New(t, app.Postgres.Conn().Conn)
	admin := builder.Profile(factory.ProfileParams{
		Alias:    "admin",
		Email:    adminEmail,
		Password: adminPassword,
		Role:     3,
	})
	user := builder.Profile(factory.ProfileParams{
		Alias:    "user",
		Email:    userEmail,
		Password: userPassword,
		Role:     1,
	})

	adminHeader := app.AuthHeader(admin.Email, adminPassword)
	userHeader := app.AuthHeader(user.Email, userPassword)

	return app, teardownTestCase, adminHeader, userHeader
}
