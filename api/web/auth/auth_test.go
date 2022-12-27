package auth

import (
	"net/http"
	"testing"

	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"
	pb_user "github.com/werbot/werbot/api/proto/user"
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/tests"
	"github.com/werbot/werbot/internal/web/jwt"
)

var (
	testHandler *tests.TestHandler
	adminInfo   *tests.UserInfo
	userInfo    *tests.UserInfo
)

func init() {
	testHandler = tests.InitTestServer("../../../.env")
	New(&web.Handler{
		App:   testHandler.App,
		Grpc:  testHandler.GRPC,
		Cache: testHandler.Cache,
		Auth:  *testHandler.Auth,
	}).Routes()
	testHandler.FinishHandler() // init finale handler for apitest

	adminInfo = testHandler.GetUserInfo(&pb_user.SignIn_Request{
		Email:    "test-admin@werbot.net",
		Password: "test-admin@werbot.net",
	})

	userInfo = testHandler.GetUserInfo(&pb_user.SignIn_Request{
		Email:    "test-user@werbot.net",
		Password: "test-user@werbot.net",
	})
}

func apiTest() *apitest.APITest {
	return apitest.New().
		//Debug().
		HandlerFunc(testHandler.Handler)
}

func Test_postSignIn(t *testing.T) {
	t.Parallel()

	testCases := map[string][]tests.TestCase{}

	testCases["ROLE_USER_UNSPECIFIED"] = []tests.TestCase{
		{
			Name:        "Without parameters",
			RequestBody: map[string]string{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateBody).
				Equal(`$.result.email`, "email is a required field").
				Equal(`$.result.password`, "password is a required field").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name:        "Error getting params",
			RequestBody: []map[string]string{{"zz": "xx"}, {"xx": "zz"}},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateBody).
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name:        "Password is blank",
			RequestBody: map[string]string{"email": "test-admin@werbot.net"},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateBody).
				Equal(`$.result.password`, "password is a required field").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name:        "Email is blank",
			RequestBody: map[string]string{"password": "test-admin@werbot.net"},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateBody).
				Equal(`$.result.email`, "email is a required field").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name:        "Invalid password",
			RequestBody: map[string]string{"email": "test-admin@werbot.net", "password": "user@werbot.net"},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgPasswordIsNotValid).
				End(),
			RespondStatus: http.StatusBadRequest,
		},
	}

	testCases["ROLE_ADMIN"] = []tests.TestCase{
		{
			Name:        "Valid admin login and password",
			RequestBody: map[string]string{"email": "test-admin@werbot.net", "password": "test-admin@werbot.net"},
			RespondBody: jsonpath.Chain().
				Present(`access_token`).
				Present(`refresh_token`).
				End(),
			RespondStatus: http.StatusOK,
		},
	}

	testCases["ROLE_USER"] = []tests.TestCase{
		{
			Name:        "Valid user login and password",
			RequestBody: map[string]string{"email": "test-user@werbot.net", "password": "test-user@werbot.net"},
			RespondBody: jsonpath.Chain().
				Present(`access_token`).
				Present(`refresh_token`).
				End(),
			RespondStatus: http.StatusOK,
		},
	}

	for role, rtc := range testCases {
		t.Run(role, func(t *testing.T) {
			for _, tc := range rtc {
				t.Run(tc.Name, func(t *testing.T) {
					apiTest().
						Post("/auth/signin").
						JSON(tc.RequestBody).
						Expect(t).
						Assert(tc.RespondBody).
						Status(tc.RespondStatus).
						End()
				})
			}
		})
	}
}

func Test_postLogout(t *testing.T) {
	t.Parallel()

	testCases := []tests.TestCase{
		{
			Name:          "Authorized user logout",
			RequestUser:   adminInfo,
			RespondStatus: http.StatusOK,
		},
		{
			Name:          "No authorized user logout",
			RequestUser:   &tests.UserInfo{},
			RespondStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			apiTest().
				Post("/auth/logout").
				Header("Authorization", "Bearer "+tc.RequestUser.Tokens.Access).
				Expect(t).
				Status(tc.RespondStatus).
				End()
		})
	}
}

func Test_postRefresh(t *testing.T) {
	t.Parallel()
	testCases := map[string][]tests.TestCase{}

	testCases["ROLE_USER_UNSPECIFIED"] = []tests.TestCase{
		{
			Name:        "Without parameters",
			RequestBody: jwt.Tokens{},
			RequestUser: &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, "token parsing error").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
	}

	testCases["ROLE_ADMIN"] = []tests.TestCase{
		{
			Name:        "Authorized user refresh",
			RequestUser: adminInfo,
			RequestBody: jwt.Tokens{
				Refresh: adminInfo.Tokens.Refresh,
			},
			RespondBody: jsonpath.Chain().
				Present(`access_token`).
				Present(`refresh_token`).
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name:        "Bad token error",
			RequestUser: adminInfo,
			RequestBody: jwt.Tokens{
				Refresh: adminInfo.Tokens.Refresh + "error",
			},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, "token parsing error").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
	}

	testCases["ROLE_USER"] = []tests.TestCase{
		{
			Name:        "Authorized user refresh",
			RequestUser: userInfo,
			RequestBody: jwt.Tokens{
				Refresh: userInfo.Tokens.Refresh,
			},
			RespondBody: jsonpath.Chain().
				Present(`access_token`).
				Present(`refresh_token`).
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name:        "Bad token error",
			RequestUser: userInfo,
			RequestBody: jwt.Tokens{
				Refresh: userInfo.Tokens.Refresh + "error",
			},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, "token parsing error").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
	}

	for role, rtc := range testCases {
		t.Run(role, func(t *testing.T) {
			for _, tc := range rtc {
				t.Run(tc.Name, func(t *testing.T) {
					apiTest().
						Post("/auth/refresh").
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

func Test_getProfile(t *testing.T) {
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
			Name:        "Retrieval of user data",
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, msgUserInfo).
				Equal(`$.result.user_id`, adminInfo.UserID).
				Equal(`$.result.user_role`, float64(3)).
				End(),
			RespondStatus: http.StatusOK,
		},
	}

	testCases["ROLE_USER"] = []tests.TestCase{
		{
			Name:        "Retrieval of user data",
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, msgUserInfo).
				Equal(`$.result.user_id`, userInfo.UserID).
				Equal(`$.result.user_role`, float64(1)).
				End(),
			RespondStatus: http.StatusOK,
		},
	}

	for role, rtc := range testCases {
		t.Run(role, func(t *testing.T) {
			for _, tc := range rtc {
				t.Run(tc.Name, func(t *testing.T) {
					apiTest().
						Get("/auth/profile").
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
