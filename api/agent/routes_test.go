package agent

import (
	"testing"

	"github.com/werbot/werbot/api/auth"
	"github.com/werbot/werbot/internal/utils/test"
)

const (
	pathAgent       = "/v1/agent"
	pathAgentScheme = "/v1/agent/scheme"
)

func setupTest(t *testing.T) (*test.APIHandler, func(t *testing.T), map[string]string, map[string]string) {
	app, teardownTestCase := test.API(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	adminHeader := map[string]string{"x-api-key": test.ConstAdminProject1ApiKey}
	userHeader := map[string]string{"x-api-key": test.ConstUserProject1ApiKey}

	return app, teardownTestCase, adminHeader, userHeader
}
