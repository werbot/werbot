package license

/*
import (
	"net/http"
	"testing"

	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"

	accountpb "github.com/werbot/werbot/api/proto/account"
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/api/web/auth"
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
	testHandler = tests.InitTestServer("../../../.env")
	auth.New(&web.Handler{
		App:   testHandler.App,
		Grpc:  testHandler.GRPC,
		Cache: testHandler.Cache,
		Auth:  *testHandler.Auth,
	}).Routes()
	authMiddleware := middleware.Auth(testHandler.Cache).Execute()
	webHandler := &web.Handler{
		App:  testHandler.App,
		Grpc: testHandler.GRPC,
		Auth: authMiddleware,
	}

	New(webHandler, internal.GetString("LICENSE_KEY_PUBLIC", "")).Routes() // add test module handler
	testHandler.FinishHandler()                                            // init finale handler for apitest

	adminInfo = testHandler.GetUserInfo(&accountpb.SignIn_Request{
		Email:    "test-admin@werbot.net",
		Password: "test-admin@werbot.net",
	})

	adminInfo = testHandler.GetUserInfo(&accountpb.SignIn_Request{
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
				Equal(`$.message`, internal.MsgUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},
		// ROLE_ADMIN - Authorized admin
		{
			Name:        "ROLE_ADMIN_getLicenseInfo_01",
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, msgLicenseInfo).
				End(),
			RespondStatus: http.StatusOK,
		},
		// ROLE_USER - Authorized user
		{
			Name:        "ROLE_USER_getLicenseInfo_01",
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgNotFound).
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
*/
