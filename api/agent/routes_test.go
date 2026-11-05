package agent

import (
	"testing"

	"github.com/werbot/werbot/api/auth"
	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
)

const (
	pathAgent       = "/v1/agent"
	pathAgentScheme = "/v1/agent/scheme"

	ownerEmail    = "user@werbot.net"
	ownerPassword = "user@werbot.net"
)

type agentEnv struct {
	owner        factory.Profile
	project      factory.Project
	projectAPI   factory.ProjectAPI
	token        factory.Token
	apiKeyHeader map[string]string
}

func setupTest(t *testing.T) (*test.APIHandler, *agentEnv, func(t *testing.T)) {
	app, teardownTestCase := test.APIWithoutFixtures(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	builder := factory.New(t, app.Postgres.Conn().Conn)

	env := &agentEnv{}
	env.owner = builder.Profile(factory.ProfileParams{
		Alias:    "user",
		Email:    ownerEmail,
		Password: ownerPassword,
		Role:     1,
	})
	env.project = builder.Project(factory.ProjectParams{OwnerID: env.owner.ID})
	env.projectAPI = builder.ProjectAPI(factory.ProjectAPIParams{ProjectID: env.project.ID})
	env.token = builder.Token(factory.TokenParams{
		Section:   int16(tokenenum.Section_agent),
		Action:    int16(tokenenum.Action_add),
		Status:    int16(tokenenum.Status_sent),
		ProjectID: env.project.ID,
		Data:      map[string]any{"scheme_type": 103},
	})

	env.apiKeyHeader = map[string]string{"x-api-key": env.projectAPI.APIKey}

	return app, env, teardownTestCase
}
