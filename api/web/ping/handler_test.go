package ping

import (
	"net/http"
	"testing"

	"github.com/steinfletcher/apitest"

	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/internal/tests"
)

var testHandler *tests.TestHandler

func init() {
	testHandler = tests.InitTestServer("../../../../.env")
	webHandler := &web.Handler{
		App: testHandler.App,
	}

	New(webHandler).Routes()    // add test module handler
	testHandler.FinishHandler() // init finale handler for apitest
}

func apiTest() *apitest.APITest {
	return apitest.New().
		//Debug().
		HandlerFunc(testHandler.Handler)
}

func TestHandler_getPing(t *testing.T) {
	t.Parallel()

	testCases := []tests.TestCase{
		{
			Name:          "Ping",
			RespondStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			apiTest().
				//Debug().
				HandlerFunc(testHandler.Handler).
				Get("/ping").
				Expect(t).
				Body("pong").
				Status(tc.RespondStatus).
				End()
		})
	}
}
