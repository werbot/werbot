package info

/*
import (
	"net/http"
	"testing"

	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"

	accountpb "github.com/werbot/werbot/api/proto/account"
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/api/auth"
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
	New(webHandler).Routes()    // add test module handler
	testHandler.FinishHandler() // init finale handler for apitest

	adminInfo = testHandler.GetUserInfo(&accountpb.SignIn_Request{
		Email:    "test-admin@werbot.net",
		Password: "test-admin@werbot.net",
	})

	userInfo = testHandler.GetUserInfo(&accountpb.SignIn_Request{
		Email:    "test-user@werbot.net",
		Password: "test-user@werbot.net",
	})
}

func apiTest() *apitest.APITest {
	return apitest.New().
		//Debug().
		HandlerFunc(testHandler.Handler)
}

func Test_getUpdate(t *testing.T) {
	t.Parallel()
	testCases := map[string][]tests.TestCase{}

	testCases["ROLE_USER_UNSPECIFIED"] = []tests.TestCase{
		{
			Name:        "Without parameters",
			RequestUser: &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},
	}

	testCases["ROLE_ADMIN"] = []tests.TestCase{
		{
			Name:        "Show versions of components",
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, msgCurrentVersions).
				End(),
			RespondStatus: http.StatusOK,
		},
	}

	testCases["ROLE_USER"] = []tests.TestCase{
		{
			Name:        "Disable show versions of components",
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgNotFound).
				End(),
			RespondStatus: http.StatusNotFound,
		},
	}

	for role, rtc := range testCases {
		t.Run(role, func(t *testing.T) {
			for _, tc := range rtc {
				t.Run(tc.Name, func(t *testing.T) {
					apiTest().
						Get("/v1/update").
						Header("Authorization", "Bearer "+tc.RequestUser.Tokens.Access).
						Expect(t).
						Assert(tc.RespondBody).
						Status(tc.RespondStatus).
						End()
				})
			}
		})
	}
}

func Test_getInfo(t *testing.T) {
	t.Parallel()
	testCases := map[string][]tests.TestCase{}

	testCases["ROLE_USER_UNSPECIFIED"] = []tests.TestCase{
		{
			Name:         "Without parameters",
			RequestParam: map[string]string{},
			RequestUser:  &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},
	}

	testCases["ROLE_ADMIN"] = []tests.TestCase{
		{
			Name:         "No send user_id",
			RequestParam: map[string]string{},
			RequestUser:  adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, msgShortInfo).
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "No send user_id",
			RequestParam: map[string]string{
				"user_id": userInfo.UserID,
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, msgShortInfo).
				Equal(`$.result.projects`, float64(1)).
				Equal(`$.result.servers`, float64(2)).
				End(),
			RespondStatus: http.StatusOK,
		},
	}

	testCases["ROLE_USER"] = []tests.TestCase{
		{
			Name:         "No send user_id",
			RequestParam: map[string]string{},
			RequestUser:  userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, msgShortInfo).
				Equal(`$.result.projects`, float64(1)).
				Equal(`$.result.servers`, float64(2)).
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "Short information",
			RequestParam: map[string]string{
				"user_id": userInfo.UserID,
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, msgShortInfo).
				Equal(`$.result.projects`, float64(1)).
				Equal(`$.result.servers`, float64(2)).
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "Bad user_id", // ignoring other user_id
			RequestParam: map[string]string{
				"user_id": adminInfo.UserID,
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, msgShortInfo).
				Equal(`$.result.projects`, float64(1)).
				Equal(`$.result.servers`, float64(2)).
				End(),
			RespondStatus: http.StatusOK,
		},
	}

	for role, rtc := range testCases {
		t.Run(role, func(t *testing.T) {
			for _, tc := range rtc {
				t.Run(tc.Name, func(t *testing.T) {
					apiTest().
						Get("/v1/info").
						QueryParams(tc.RequestParam.(map[string]string)).
						Header("Authorization", "Bearer "+tc.RequestUser.Tokens.Access).
						Expect(t).
						Assert(tc.RespondBody).
						Status(tc.RespondStatus).
						End()
				})
			}
		})
	}
}

func Test_getVersion(t *testing.T) {
	t.Parallel()
	testCases := map[string][]tests.TestCase{}

	testCases["ROLE_USER_UNSPECIFIED"] = []tests.TestCase{
		{
			Name:        "Without parameters",
			RequestUser: &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},
	}

	testCases["ROLE_ADMIN"] = []tests.TestCase{
		{
			Name:        "Without parameters",
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, msgAPIVersion).
				End(),
			RespondStatus: http.StatusOK,
		},
	}

	testCases["ROLE_USER"] = []tests.TestCase{
		{
			Name:        "Without parameters",
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, msgAPIVersion).
				End(),
			RespondStatus: http.StatusOK,
		},
	}

	for role, rtc := range testCases {
		t.Run(role, func(t *testing.T) {
			for _, tc := range rtc {
				t.Run(tc.Name, func(t *testing.T) {
					apiTest().
						Get("/v1/version").
						JSON(tc.RequestBody).
						Header("Authorization", "Bearer "+tc.RequestUser.Tokens.Access).
						Expect(t).
						Assert(tc.RespondBody).
						Status(tc.RespondStatus).
						End()
				})
			}
		})
	}
}
*/
