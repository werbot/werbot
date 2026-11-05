package key

import (
	"testing"
	"time"

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
	app, teardownTestCase := test.APIWithoutFixtures(t)
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
