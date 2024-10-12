package event

import (
	"testing"

	"github.com/werbot/werbot/api/auth"
	"github.com/werbot/werbot/internal/utils/test"
)

const (
	pathEvent        = "/v1/event"
	pathEventProfile = pathEvent + "/profile"
	pathEventProject = pathEvent + "/project"
	pathEventScheme  = pathEvent + "/scheme"
)

func setupTest(t *testing.T) (*test.APIHandler, func(t *testing.T), map[string]string, map[string]string) {
	app, teardownTestCase := test.API(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	adminHeader, userHeader := app.TestProfileAuth()

	return app, teardownTestCase, adminHeader, userHeader
}
