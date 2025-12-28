package token

/*
import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_profileTokens(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathTokensProfile,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       pathTokensProfile,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":            float64(200),
				"message":         "Profile tokens",
				"result.total":    float64(2),
				"result.tokens.3": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       pathTokensProfile + "?limit=1&offset=0",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":            float64(200),
				"message":         "Profile tokens",
				"result.total":    float64(2),
				"result.tokens.0": "*",
				"result.tokens.1": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       pathTokensProfile + "?profile_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgTokenNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       pathTokensProfile + "?profile_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Profile tokens",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER:
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       pathTokensProfile,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":            float64(200),
				"message":         "Profile tokens",
				"result.total":    float64(1),
				"result.tokens.3": nil,
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       pathTokensProfile + "?limit=1&offset=0",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":            float64(200),
				"message":         "Profile tokens",
				"result.total":    float64(1),
				"result.tokens.0": "*",
				"result.tokens.1": nil,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: ignored profile UUID
			Name:       "test2_03",
			Method:     http.MethodGet,
			Path:       pathTokensProfile + "?profile_id=" + test.ConstFakeID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Profile tokens",
				"result.total": float64(1),
			},
			RequestHeaders: userHeader,
		},
		{ // USER: ignored profile UUID
			Name:       "test2_04",
			Method:     http.MethodGet,
			Path:       pathTokensProfile + "?profile_id=" + test.ConstAdminID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Profile tokens",
				"result.total": float64(1),
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_addProfileToken(t *testing.T) {
	app, teardownTestCase, _ , _ := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPost,
			Path:       pathTokensProfile,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_updateProfileToken(t *testing.T) {
	app, teardownTestCase, _ , _ := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPatch,
			Path:       pathTokensProfile,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_deleteProfileToken(t *testing.T) {
	app, teardownTestCase, _, _ := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodDelete,
			Path:       pathTokensProfile,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}
*/
