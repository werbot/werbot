package ping

import (
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
)

func setupTest(t *testing.T) (*test.APIHandler, func(t *testing.T)) {
	app, teardownTestCase := test.API(t)
	New(app.Handler).Routes()
	app.AddRoute404()

	return app, teardownTestCase
}
