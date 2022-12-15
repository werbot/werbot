package license

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
	internal.LoadConfig("../../../../.env") // only for LICENSE_KEY_PUBLIC

	testHandler = tests.InitTestServer("../../../../.env")
	authMiddleware := middleware.Auth(testHandler.Cache).Execute()
	New(testHandler.App, testHandler.GRPC, authMiddleware, internal.GetString("LICENSE_KEY_PUBLIC", "")).Routes() // add test module handler
	testHandler.FinishHandler()                                                                                   // init finale handler for apitest

	adminInfo = testHandler.GetUserInfo(&pb.SignIn_Request{
		Email:    "test-admin@werbot.net",
		Password: "test-admin@werbot.net",
	})

	adminInfo = testHandler.GetUserInfo(&pb.SignIn_Request{
		Email:    "test-user@werbot.net",
		Password: "test-user@werbot.net",
	})
}

func apiTest() *apitest.APITest {
	return apitest.New().
		//Debug().
		HandlerFunc(testHandler.Handler)
}

func TestHandler_getLicenseInfo(t *testing.T) {
	t.Parallel()

	testCases := []tests.TestCase{
		// Unauthorized user
		{
			Name:        "getLicenseInfo_01",
			RequestUser: &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.ErrUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},
		// ROLE_ADMIN - Authorized admin
		{
			Name:        "ROLE_ADMIN_getLicenseInfo_01",
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "License information").
				End(),
			RespondStatus: http.StatusOK,
		},
		// ROLE_USER - Authorized user
		{
			Name:        "ROLE_USER_getLicenseInfo_01",
			RequestUser: userInfo,
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
				HandlerFunc(testHandler.Handler).
				Get("/v1/license/info").
				Header("Authorization", "Bearer "+tc.RequestUser.Tokens.Access).
				Expect(t).
				Assert(tc.RespondBody).
				Status(tc.RespondStatus).
				End()
		})
	}
}
