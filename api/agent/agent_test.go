package agent

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_auth(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{
			Name:       "test0_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathAgent, "auth"),
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{
			Name:       "test0_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathAgent, "auth", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgTokenNotFound,
			},
		},
		{
			Name:       "test0_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathAgent, "auth", env.token.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Auth data",
				"result.api_key":     env.projectAPI.APIKey,
				"result.api_secret":  env.projectAPI.APISecret,
				"result.scheme_type": float64(103),
			},
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_addScheme(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{
			Name:       "test0_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathAgentScheme),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:           "test1_01",
			Method:         http.MethodPost,
			Path:           test.PathGluing(pathAgentScheme),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: env.apiKeyHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathAgentScheme, test.ConstFakeID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.address": "value is required",
				"result.port":    "value is required",
				"result.login":   "value is required",
			},
			RequestHeaders: env.apiKeyHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathAgentScheme, test.ConstFakeID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"address": "127.0.0.255",
				"port":    float64(2922),
				"login":   "ubuntu",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgTokenNotFound,
			},
			RequestHeaders: env.apiKeyHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathAgentScheme, env.token.ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"address": "127.0.0.255",
				"port":    float64(2922),
				"login":   "ubuntu",
			},
			Body: test.BodyTable{
				"code":              float64(200),
				"message":           "Scheme data",
				"result.public_key": "*",
				"result.scheme_id":  "*",
			},
			RequestHeaders: env.apiKeyHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}
