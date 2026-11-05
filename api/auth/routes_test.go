package auth

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
)

const (
	pathAuth              = "/auth"
	pathAuthSignIn        = pathAuth + "/signin"
	pathAuthLogout        = pathAuth + "/logout"
	pathAuthRefresh       = pathAuth + "/refresh"
	pathAuthPasswordReset = pathAuth + "/password_reset"
)

// authEnv holds the fixture-free admin and user profiles created by setupTest.
type authEnv struct {
	admin         factory.Profile
	adminPassword string
	user          factory.Profile
	userPassword  string
}

func setupTest(t *testing.T) (*test.APIHandler, *authEnv, func(t *testing.T)) {
	app, teardownTestCase := test.API(t)
	New(app.Handler).Routes()
	app.AddRoute404()

	builder := factory.New(t, app.Postgres.Conn().Conn)

	adminPassword := "admin@werbot.net"
	admin := builder.Profile(factory.ProfileParams{
		Email:    "admin@werbot.net",
		Password: adminPassword,
		Role:     3, // admin
	})

	userPassword := "user@werbot.net"
	user := builder.Profile(factory.ProfileParams{
		Email:    "user@werbot.net",
		Password: userPassword,
		Role:     1, // user
	})

	return app, &authEnv{
		admin:         admin,
		adminPassword: adminPassword,
		user:          user,
		userPassword:  userPassword,
	}, teardownTestCase
}

func TestSetupCreatesAdminAndUserProfiles(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	require.NotEmpty(t, env.admin.ID)
	require.NotEmpty(t, env.admin.Email)
	require.NotEmpty(t, env.adminPassword)
	require.NotEmpty(t, env.user.ID)
	require.NotEmpty(t, env.user.Email)
	require.NotEmpty(t, env.userPassword)

	adminAuth := app.GetProfileAuth(env.admin.Email, env.adminPassword)
	require.NotEmpty(t, adminAuth.Tokens.Access)

	userAuth := app.GetProfileAuth(env.user.Email, env.userPassword)
	require.NotEmpty(t, userAuth.Tokens.Access)
}
