package auth_test

import (
	"net/http"
	"testing"

	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"
	pb_user "github.com/werbot/werbot/internal/grpc/proto/user"
	"github.com/werbot/werbot/internal/message"
	"github.com/werbot/werbot/internal/tests"
	"github.com/werbot/werbot/internal/web/httputil"
)

var (
	testHandler *tests.TestHandler
	adminInfo   *tests.UserInfo
	userInfo    *tests.UserInfo
)

func init() {
	testHandler = tests.InitTestServer("../../../../.vscode/config/.env.taco")
	testHandler.FinishHandler() // init finale handler for apitest

	adminInfo = testHandler.GetUserInfo(&pb_user.AuthUser_Request{
		Email:    "test-admin@werbot.net",
		Password: "test-admin@werbot.net",
	})

	userInfo = testHandler.GetUserInfo(&pb_user.AuthUser_Request{
		Email:    "test-user@werbot.net",
		Password: "test-user@werbot.net",
	})
}

func apiTest() *apitest.APITest {
	return apitest.New().
		//Debug().
		HandlerFunc(testHandler.Handler)
}

func Test_postLogin(t *testing.T) {
	t.Parallel()

	testCases := map[string][]tests.TestCase{}

	testCases["ROLE_USER_UNSPECIFIED"] = []tests.TestCase{
		{
			Name:        "Without parameters",
			RequestBody: map[string]string{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, message.ErrValidateBodyParams).
				Equal(`$.result.email`, "Email is a required field").
				Equal(`$.result.password`, "Password is a required field").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name:        "Error getting params",
			RequestBody: []map[string]string{{"zz": "xx"}, {"xx": "zz"}},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, message.ErrValidateBodyParams).
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name:        "Password is blank",
			RequestBody: map[string]string{"email": "test-admin@werbot.net"},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, message.ErrValidateBodyParams).
				Equal(`$.result.password`, "Password is a required field").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name:        "Email is blank",
			RequestBody: map[string]string{"password": "test-admin@werbot.net"},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, message.ErrValidateBodyParams).
				Equal(`$.result.email`, "Email is a required field").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name:        "Invalid password",
			RequestBody: map[string]string{"email": "test-admin@werbot.net", "password": "user@werbot.net"},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, message.ErrInvalidPassword).
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
						Post("/auth/login").
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
				Header("Authorization", "Bearer "+tc.RequestUser.Tokens.AccessToken).
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
			RequestBody: httputil.RefreshToken{},
			RequestUser: &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, "Token parsing error").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
	}

	testCases["ROLE_ADMIN"] = []tests.TestCase{
		{
			Name:        "Authorized user refresh",
			RequestUser: adminInfo,
			RequestBody: httputil.RefreshToken{
				Token: adminInfo.Tokens.RefreshToken,
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
			RequestBody: httputil.RefreshToken{
				Token: adminInfo.Tokens.RefreshToken + "error",
			},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, "Token parsing error").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
	}

	testCases["ROLE_USER"] = []tests.TestCase{
		{
			Name:        "Authorized user refresh",
			RequestUser: userInfo,
			RequestBody: httputil.RefreshToken{
				Token: userInfo.Tokens.RefreshToken,
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
			RequestBody: httputil.RefreshToken{
				Token: userInfo.Tokens.RefreshToken + "error",
			},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, "Token parsing error").
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
						Header("Authorization", "Bearer "+tc.RequestUser.Tokens.AccessToken).
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
				Equal(`$.message`, message.ErrUnauthorized).
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
				Equal(`$.message`, "User information").
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
				Equal(`$.message`, "User information").
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
						Header("Authorization", "Bearer "+tc.RequestUser.Tokens.AccessToken).
						Expect(t).
						Assert(tc.RespondBody).
						Status(tc.RespondStatus).
						End()
				})
			}
		})
	}
}
