package project

import (
	"testing"

	"github.com/werbot/werbot/api/auth"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
)

const (
	pathProjects = "/v1/projects"

	adminPassword = "admin@werbot.net"
	userPassword  = "user@werbot.net"
)

type projectEnv struct {
	admin, user             factory.Profile
	adminHeader, userHeader map[string]string

	adminProject1, adminProject2, adminProject7 factory.Project
	userProject3, userProject13                 factory.Project
	adminProject1API, adminProject2API          factory.ProjectAPI
	userProject3API, userProject13API           factory.ProjectAPI
}

func setupTest(t *testing.T) (*test.APIHandler, *projectEnv, func(t *testing.T)) {
	app, teardownTestCase := test.APIWithoutFixtures(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	builder := factory.New(t, app.Postgres.Conn().Conn)

	env := &projectEnv{}
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

	env.adminProject7 = builder.Project(factory.ProjectParams{ID: "10000000-0000-4000-8000-000000000007", OwnerID: env.admin.ID, Title: "project7", Alias: "68rwRW"})
	env.adminProject2 = builder.Project(factory.ProjectParams{ID: "30000000-0000-4000-8000-000000000002", OwnerID: env.admin.ID, Title: "project2", Alias: "U2g7uG"})
	env.adminProject1 = builder.Project(factory.ProjectParams{ID: "20000000-0000-4000-8000-000000000001", OwnerID: env.admin.ID, Title: "project1", Alias: "Y93iyI"})

	env.userProject13 = builder.Project(factory.ProjectParams{ID: "10000000-0000-4000-8000-000000000013", OwnerID: env.user.ID, Title: "project13", Alias: "9Yyz9Z"})
	env.userProject3 = builder.Project(factory.ProjectParams{ID: "20000000-0000-4000-8000-000000000003", OwnerID: env.user.ID, Title: "project3", Alias: "C8cXx0"})

	env.adminProject1API = builder.ProjectAPI(factory.ProjectAPIParams{ProjectID: env.adminProject1.ID})
	env.adminProject2API = builder.ProjectAPI(factory.ProjectAPIParams{ProjectID: env.adminProject2.ID})
	env.userProject3API = builder.ProjectAPI(factory.ProjectAPIParams{ProjectID: env.userProject3.ID})
	env.userProject13API = builder.ProjectAPI(factory.ProjectAPIParams{ProjectID: env.userProject13.ID})

	env.adminHeader = app.AuthHeader(env.admin.Email, adminPassword)
	env.userHeader = app.AuthHeader(env.user.Email, userPassword)

	return app, env, teardownTestCase
}
