package project

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

type projectListItem struct {
	ProjectID string `json:"project_id"`
	OwnerID   string `json:"owner_id"`
	Title     string `json:"title"`
	Alias     string `json:"alias"`
}

// assertProjectInList fetches a project list and asserts that want is present.
// The list query has no ORDER BY, so the test must not depend on row indexes.
func assertProjectInList(t *testing.T, app *test.APIHandler, path string, headers map[string]string, want projectListItem) {
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
			Projects []projectListItem `json:"projects"`
		} `json:"result"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&body))

	for _, project := range body.Result.Projects {
		if project.ProjectID == want.ProjectID {
			assert.Equal(t, want, project)
			return
		}
	}

	require.Failf(t, "project not found", "project %s is missing from %s", want.ProjectID, path)
}
