package ping

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_ping(t *testing.T) {
	app, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // GET ping to get a answer
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       "/ping",
			StatusCode: 200,
		},
		{ // POST ping method not allowed
			Name:       "test0_02",
			Method:     http.MethodPost,
			Path:       "/ping",
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}
