package auth

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/utils/fsutil"
)

func TestHandler_signin(t *testing.T) {
	app, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // incorrect method
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathAuthSignIn,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // clear request
			Name:       "test0_02",
			Method:     http.MethodPost,
			Path:       pathAuthSignIn,
			StatusCode: 400,
			Body: test.BodyTable{
				"code": float64(400),
			},
		},
		{ // clear body request
			Name:        "test0_03",
			Method:      http.MethodPost,
			Path:        pathAuthSignIn,
			StatusCode:  400,
			RequestBody: test.BodyTable{},
			Body: test.BodyTable{
				"code":            float64(400),
				"result.email":    "value is required",
				"result.password": "value is required",
			},
		},
		{ // clear body request
			Name:       "test0_04",
			Method:     http.MethodPost,
			Path:       pathAuthSignIn,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"email":    "email",
				"password": "123",
			},
			Body: test.BodyTable{
				"code":            float64(400),
				"result.email":    "value must be a valid email address",
				"result.password": "value length must be at least 8 characters",
			},
		},
		{ // email requests only
			Name:       "test0_05",
			Method:     http.MethodPost,
			Path:       pathAuthSignIn,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"email": test.ConstAdminEmail,
			},
			Body: test.BodyTable{
				"code":            float64(400),
				"result.password": "value is required",
			},
		},
		{ // password request only
			Name:       "test0_06",
			Method:     http.MethodPost,
			Path:       pathAuthSignIn,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"password": test.ConstAdminEmail,
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"result.email": "value is required",
			},
		},
		{ // incorrect login
			Name:       "test0_07",
			Method:     http.MethodPost,
			Path:       pathAuthSignIn,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"email":    test.ConstAdminEmail,
				"password": crypto.NewPassword(1, false),
			},
			Body: test.BodyTable{
				"code":            float64(400),
				"result.password": "value length must be at least 8 characters",
			},
		},
		{ // incorrect login or password
			Name:       "test0_08",
			Method:     http.MethodPost,
			Path:       pathAuthSignIn,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"email":    test.ConstUnknownEmail,
				"password": test.ConstUnknownPassword,
			},
			Body: test.BodyNotFound,
		},
		{ // ADMIN: authorization
			Name:       "test1_01",
			Method:     http.MethodPost,
			Path:       pathAuthSignIn,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"email":    test.ConstAdminEmail,
				"password": test.ConstAdminPassword,
			},
			Body: test.BodyTable{
				"code":                 float64(200),
				"message":              "Tokens",
				"result.access_token":  "*",
				"result.refresh_token": "*",
			},
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_logout(t *testing.T) {
	app, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	adminAuth := app.GetProfileAuth(test.ConstAdminEmail, test.ConstAdminPassword)
	adminHeader := test.HeadersTable{"Authorization": "Bearer " + adminAuth.Tokens.Access}

	userAuth := app.GetProfileAuth(test.ConstUserEmail, test.ConstUserPassword)
	userHeader := test.HeadersTable{"Authorization": "Bearer " + userAuth.Tokens.Access}

	testTable := []test.APITable{
		{ // incorrect method
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathAuthLogout,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodPost,
			Path:       pathAuthLogout,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: authorized request
			Name:       "test1_01",
			Method:     http.MethodPost,
			Path:       pathAuthLogout,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Successful logout",
			},
			RequestHeaders: adminHeader,
		},
		{ // USER: authorized request
			Name:       "test2_01",
			Method:     http.MethodPost,
			Path:       pathAuthLogout,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Successful logout",
			},
			RequestHeaders: userHeader,
		},
		// TODO add other test cases to logout
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_refresh(t *testing.T) {
	app, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	adminAuth := app.GetProfileAuth(test.ConstAdminEmail, test.ConstAdminPassword)
	adminHeader := test.HeadersTable{"Authorization": "Bearer " + adminAuth.Tokens.Access}

	testTable := []test.APITable{
		{ // incorrect method
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathAuthRefresh,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:           "test0_02",
			Method:         http.MethodPost,
			Path:           pathAuthRefresh,
			StatusCode:     400,
			Body:           test.BodyInvalidArgument,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:        "test1_01",
			Method:      http.MethodPost,
			Path:        pathAuthRefresh,
			StatusCode:  400,
			RequestBody: test.BodyTable{},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  "Impossible to parse the key",
			},
			RequestHeaders: adminHeader,
		},

		{ // ADMIN: request with empty token
			Name:       "test1_02",
			Method:     http.MethodPost,
			Path:       pathAuthRefresh,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"refresh_token": "",
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  "Impossible to parse the key",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken token
			Name:       "test1_03",
			Method:     http.MethodPost,
			Path:       pathAuthRefresh,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"refresh": crypto.NewPassword(3, false),
			},
			Body: test.BodyTable{
				"code":   float64(400),
				"result": "Impossible to parse the key",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:       "test1_04",
			Method:     http.MethodPost,
			Path:       pathAuthRefresh,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"refresh": adminAuth.Tokens.Refresh,
			},
			Body: test.BodyTable{
				"code":                 float64(200),
				"message":              "Tokens",
				"result.access_token":  "*",
				"result.refresh_token": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with an expired token
			Name:       "test1_05",
			Method:     http.MethodPost,
			Path:       pathAuthRefresh,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"refresh": fsutil.RemoveByteLineBreak(fsutil.MustReadFile("../../fixtures/auth/admin/expired_refresh_token")),
			},
			Body:           test.BodyInvalidArgument,
			RequestHeaders: adminHeader,
		},
		// TODO add other test cases to refresh token
	}

	test.RunCaseAPITests(t, app, testTable)
}
