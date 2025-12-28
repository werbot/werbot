package token

/*
import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_token(t *testing.T) {
	app, teardownTestCase, adminHeader, _ := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathTokens,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ //
			Name:       "test0_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathTokens, "test"),
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ //
			Name:       "test0_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathTokens, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgTokenNotFound,
			},
		},
		{ //
			Name:       "test0_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathTokens, "3c818d7c-72f3-4518-8eaa-755585192f21"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                float64(200),
				"message":             "Token",
				"result.action":       float64(2),
				"result.status":       float64(1),
				"profile_id":          nil,
				"result.expired_at":   "*",
				"result.updated_at":   nil,
				"result.created_at":   "*",
				"result.data.email":   "user1@werbot.net",
				"result.data.name":    "Harrison",
				"result.data.surname": "Bowling",
			},
		},

		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathTokens, "3c818d7c-72f3-4518-8eaa-755585192f21"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                float64(200),
				"message":             "Token",
				"result.action":       float64(2),
				"result.status":       float64(1),
				"result.profile_id":   "*", // visible only in admin mode
				"result.expired_at":   "*",
				"result.updated_at":   nil,
				"result.created_at":   "*",
				"result.data.email":   "user1@werbot.net",
				"result.data.name":    "Harrison",
				"result.data.surname": "Bowling",
			},
			RequestHeaders: adminHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}
*/
