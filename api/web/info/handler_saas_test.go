//go:build saas

package info

import (
	"net/http"
	"testing"

	jsonpath "github.com/steinfletcher/apitest-jsonpath"
	"github.com/werbot/werbot/internal/tests"
)

func Test_getUpdateVersion(t *testing.T) {
	t.Parallel()

	testCases := []tests.TestCase{
		{
			Name: "Last actual version of components",
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "actual versions of components").
				End(),
			RespondStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			apiTest().
				Get("/v1/update/version").
				JSON(tc.RequestBody).
				Expect(t).
				Assert(tc.RespondBody).
				Status(tc.RespondStatus).
				End()
		})
	}
}
