package profile

import (
	"testing"
	"time"

	"github.com/werbot/werbot/api/auth"
	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
)

const (
	pathProfiles = "/v1/profiles"

	adminPassword = "admin@werbot.net"
	userPassword  = "user@werbot.net"
)

type profileEnv struct {
	admin, user             factory.Profile
	adminHeader, userHeader map[string]string
	deleteToken             factory.Token
	expiredDeleteToken      factory.Token
}

func setupTest(t *testing.T) (*test.APIHandler, *profileEnv, func(t *testing.T)) {
	app, teardownTestCase := test.APIWithoutFixtures(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	builder := factory.New(t, app.Postgres.Conn().Conn)

	env := &profileEnv{}
	env.admin = builder.Profile(factory.ProfileParams{
		Alias:     "admin",
		Name:      "Penny",
		Surname:   "Hoyle",
		Email:     adminPassword,
		Password:  adminPassword,
		Role:      3,
		UpdatedAt: factory.Time(time.Now()),
	})
	env.user = builder.Profile(factory.ProfileParams{
		Alias:    "user",
		Name:     "Carly",
		Surname:  "Bender",
		Email:    userPassword,
		Password: userPassword,
		Role:     1,
	})

	adminProject1 := builder.Project(factory.ProjectParams{OwnerID: env.admin.ID})
	adminProject2 := builder.Project(factory.ProjectParams{OwnerID: env.admin.ID})
	builder.Scheme(factory.SchemeParams{ProjectID: adminProject1.ID})
	builder.Scheme(factory.SchemeParams{ProjectID: adminProject1.ID})
	builder.Scheme(factory.SchemeParams{ProjectID: adminProject2.ID})
	builder.PublicKey(factory.PublicKeyParams{ProfileID: env.admin.ID})

	userProject := builder.Project(factory.ProjectParams{OwnerID: env.user.ID})
	builder.Scheme(factory.SchemeParams{ProjectID: userProject.ID})
	builder.Scheme(factory.SchemeParams{ProjectID: userProject.ID})
	builder.PublicKey(factory.PublicKeyParams{ProfileID: env.user.ID})
	builder.PublicKey(factory.PublicKeyParams{ProfileID: env.user.ID})

	env.adminHeader = app.AuthHeader(env.admin.Email, adminPassword)
	env.userHeader = app.AuthHeader(env.user.Email, userPassword)

	past := time.Now().Add(-48 * time.Hour)
	// The expired token belongs to the admin, who survives the user delete
	// case, so the 404 can only come from token expiry.
	env.expiredDeleteToken = builder.Token(factory.TokenParams{
		Section:   int16(tokenenum.Section_profile),
		Action:    int16(tokenenum.Action_delete),
		Status:    int16(tokenenum.Status_sent),
		ProfileID: env.admin.ID,
		CreatedAt: factory.Time(past),
		ExpiredAt: factory.Time(time.Now().Add(-time.Hour)),
	})
	env.deleteToken = builder.Token(factory.TokenParams{
		Section:   int16(tokenenum.Section_profile),
		Action:    int16(tokenenum.Action_delete),
		Status:    int16(tokenenum.Status_sent),
		ProfileID: env.user.ID,
	})

	return app, env, teardownTestCase
}
