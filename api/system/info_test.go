package system

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_update(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathSystemUpdate,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:           "test1_01",
			Method:         http.MethodGet,
			Path:           pathSystemUpdate,
			StatusCode:     200,
			Body:           test.BodyTable{"code": float64(200), "message": "Updates"},
			RequestHeaders: env.adminHeader,
		},
		{ // USER: request without parameters
			Name:           "test2_01",
			Method:         http.MethodGet,
			Path:           pathSystemUpdate,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: env.userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_info(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathSystemInfo,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       pathSystemInfo,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Short",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: request with show global information
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       pathSystemInfo + "?profile_id=" + test.ConstFakeID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":            float64(200),
				"message":         "Short",
				"result.profiles": float64(2),
				"result.projects": float64(3),
				"result.schemes":  float64(5),
			}, RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: request with profile UUID
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       pathSystemInfo + "?profile_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":            float64(200),
				"message":         "Short",
				"result.projects": float64(1),
				"result.schemes":  float64(2),
			}, RequestHeaders: env.adminHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       pathSystemInfo,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":            float64(200),
				"message":         "Short",
				"result.projects": float64(1),
				"result.schemes":  float64(2),
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: request without parameters with fake profile UUID
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       pathSystemInfo + "?profile_id=" + test.ConstFakeID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":            float64(200),
				"message":         "Short",
				"result.projects": float64(1),
				"result.schemes":  float64(2),
			},
			RequestHeaders: env.userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_version(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathSystemVersion,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       pathSystemVersion,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":       float64(200),
				"message":    "Apps version",
				"result.api": "0.0.1 (00000000)",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       pathSystemVersion,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":       float64(200),
				"message":    "Apps version",
				"result.api": "0.0.1 (00000000)",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       pathSystemVersion,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":       float64(200),
				"message":    "Apps version",
				"result.api": "0.0.1 (00000000)",
			},
			RequestHeaders: env.userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}
