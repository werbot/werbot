package system

import (
	"testing"

	"github.com/werbot/werbot/api/auth"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
)

const (
	pathSystem          = "/v1/system"
	pathSystemInfo      = pathSystem + "/info"
	pathSystemUpdate    = pathSystem + "/update"
	pathSystemVersion   = pathSystem + "/version"
	pathSystemMyIP      = pathSystem + "/myip"
	pathSystemCountries = pathSystem + "/countries"

	adminPassword = "admin@werbot.net"
	userPassword  = "user@werbot.net"
)

type systemEnv struct {
	admin, user             factory.Profile
	adminHeader, userHeader map[string]string
}

func setupTest(t *testing.T) (*test.APIHandler, *systemEnv, func(t *testing.T)) {
	app, teardownTestCase := test.APIWithoutFixtures(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	builder := factory.New(t, app.Postgres.Conn().Conn)

	env := &systemEnv{}
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

	adminProject1 := builder.Project(factory.ProjectParams{OwnerID: env.admin.ID})
	adminProject2 := builder.Project(factory.ProjectParams{OwnerID: env.admin.ID})
	builder.Scheme(factory.SchemeParams{ProjectID: adminProject1.ID})
	builder.Scheme(factory.SchemeParams{ProjectID: adminProject1.ID})
	builder.Scheme(factory.SchemeParams{ProjectID: adminProject2.ID})

	userProject := builder.Project(factory.ProjectParams{OwnerID: env.user.ID})
	builder.Scheme(factory.SchemeParams{ProjectID: userProject.ID})
	builder.Scheme(factory.SchemeParams{ProjectID: userProject.ID})

	env.adminHeader = app.AuthHeader(env.admin.Email, adminPassword)
	env.userHeader = app.AuthHeader(env.user.Email, userPassword)

	return app, env, teardownTestCase
}
