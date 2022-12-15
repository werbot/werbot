package member

import (
	"net/http"
	"testing"

	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"
	pb "github.com/werbot/werbot/api/proto/user"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/tests"
	"github.com/werbot/werbot/internal/web/middleware"
)

var (
	testHandler *tests.TestHandler
	adminInfo   *tests.UserInfo
	userInfo    *tests.UserInfo
)

func init() {
	testHandler = tests.InitTestServer("../../../../.env.taco")
	authMiddleware := middleware.Auth(testHandler.Cache).Execute()
	New(testHandler.App, testHandler.GRPC, authMiddleware).Routes() // add test module handler
	testHandler.FinishHandler()

	adminInfo = testHandler.GetUserInfo(&pb.SignIn_Request{
		Email:    "test-admin@werbot.net",
		Password: "test-admin@werbot.net",
	})

	userInfo = testHandler.GetUserInfo(&pb.SignIn_Request{
		Email:    "test-user@werbot.net",
		Password: "test-user@werbot.net",
	})
}

func apiTest() *apitest.APITest {
	return apitest.New().
		Debug().
		HandlerFunc(testHandler.Handler)
}

func TestHandler_getMembers(t *testing.T) {
	t.Parallel()

	testCases := []tests.TestCase{
		// Unauthorized user error
		{
			Name:        "getMembers_01",
			RequestUser: &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.ErrUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},

		// ROLE_ADMIN - Error validating body params
		{
			Name:        "ROLE_ADMIN_getMembers_01",
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.ErrValidateBodyParams).
				End(),
			RespondStatus: http.StatusBadRequest,
		},

		// ROLE_ADMIN - Submitted in wrong format
		{
			Name:        "ROLE_ADMIN_getMembers_02",
			RequestBody: map[string]string{"project_id": "5d013c61-83d1-4b59-b430-1edfd5f2b8d9"},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.ErrValidateBodyParams).
				Equal(`$.result.projectid`, "ProjectId is a required field").
				End(),
			RespondStatus: http.StatusBadRequest,
		},

		// ROLE_ADMIN - List of servers available in this project
		// Project owner, administrator
		{
			Name:        "ROLE_ADMIN_getMembers_03",
			RequestBody: map[string]string{"project_id": "ROLE_ADMIN_getMembers_03"},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "Servers available in this project").
				End(),
			RespondStatus: http.StatusOK,
		},

		// ROLE_ADMIN - NotFound - List of servers available in this project
		// Project owner, user
		{
			Name:        "ROLE_ADMIN_getMembers_04",
			RequestBody: map[string]int{"project_id": 3, "owner_id": 2},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.ErrNotFound).
				End(),
			RespondStatus: http.StatusNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			apiTest().
				Get("/v1/members").
				JSON(tc.RequestBody).
				Header("Authorization", "Bearer "+tc.RequestUser.Tokens.Access).
				Expect(t).
				Assert(tc.RespondBody).
				Status(tc.RespondStatus).
				End()
		})
	}
}
