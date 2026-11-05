package project

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/pkg/crypto"
)

func TestHandler_projectKeys(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys"),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID, "keys"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                      float64(200),
				"message":                   "Project keys",
				"result.total":              float64(1),
				"result.keys.0.key_id":      env.adminProject1API.ID,
				"result.keys.0.online":      true,
				"result.keys.0.key":         env.adminProject1API.APIKey,
				"result.keys.0.secret":      env.adminProject1API.APISecret,
				"result.keys.0.locked_at":   nil,
				"result.keys.0.archived_at": nil,
				"result.keys.0.updated_at":  nil,
				"result.keys.0.created_at":  "*",
				// --
				"result.keys.1.key_id": nil,
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID, "keys") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID, "keys") + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                      float64(200),
				"message":                   "Project keys",
				"result.total":              float64(1),
				"result.keys.0.key_id":      env.userProject3API.ID,
				"result.keys.0.key":         env.userProject3API.APIKey,
				"result.keys.0.secret":      env.userProject3API.APISecret,
				"result.keys.0.online":      true,
				"result.keys.0.locked_at":   nil,
				"result.keys.0.archived_at": nil,
				"result.keys.0.updated_at":  nil,
				"result.keys.0.created_at":  "*",
				// --
				"result.keys.1.key_id": nil,
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER:
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID, "keys"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID, "keys"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                      float64(200),
				"message":                   "Project keys",
				"result.total":              float64(1),
				"result.keys.0.key_id":      env.userProject3API.ID,
				"result.keys.0.key":         env.userProject3API.APIKey,
				"result.keys.0.secret":      env.userProject3API.APISecret,
				"result.keys.0.online":      true,
				"result.keys.0.locked_at":   nil,
				"result.keys.0.archived_at": nil,
				"result.keys.0.updated_at":  nil,
				"result.keys.0.created_at":  "*",
				// --
				"result.keys.1.key_id": nil,
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys") + "?owner_id=" + env.admin.ID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_projectKey(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		// public section
		{ // unauthorized request with fake api key
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, "key", crypto.NewPassword(37, false)),
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request with valid api key
			Name:       "test0_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, "key", env.adminProject1API.APIKey),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":              float64(200),
				"message":           "Project key",
				"result.project_id": env.adminProject1.ID,
			},
		},

		// private section
		{ // unauthorized request
			Name:       "test0_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys", env.adminProject1API.ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},

		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID, "keys", env.adminProject1API.ID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:           "test1_03",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathProjects, env.adminProject1.ID, "test", env.adminProject1API.ID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys", env.adminProject1API.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Project key",
				"result.key":         env.adminProject1API.APIKey,
				"result.secret":      env.adminProject1API.APISecret,
				"result.online":      true,
				"result.locked_at":   nil,
				"result.archived_at": nil,
				"result.updated_at":  nil,
				"result.created_at":  "*",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID, "keys", env.userProject3API.ID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID, "keys", env.userProject3API.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_07",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID, "keys", env.userProject3API.ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Project key",
				"result.key":         env.userProject3API.APIKey,
				"result.secret":      env.userProject3API.APISecret,
				"result.online":      true,
				"result.locked_at":   nil,
				"result.archived_at": nil,
				"result.updated_at":  nil,
				"result.created_at":  "*",
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER:
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID, "keys", env.userProject3API.ID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID, "keys", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:           "test2_03",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathProjects, env.userProject3.ID, "test", env.userProject3API.ID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID, "keys", env.userProject3API.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Project key",
				"result.key":         env.userProject3API.APIKey,
				"result.secret":      env.userProject3API.APISecret,
				"result.online":      true,
				"result.locked_at":   nil,
				"result.archived_at": nil,
				"result.updated_at":  nil,
				"result.created_at":  "*",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys", env.adminProject1API.ID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys", env.adminProject1API.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_07",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys", env.adminProject1API.ID) + "?owner_id=" + env.admin.ID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_addProjectKey(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys"),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID, "keys"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":          float64(200),
				"message":       "Project added",
				"result.key_id": "*",
				"result.key":    "*",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID, "keys") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID, "keys") + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":          float64(200),
				"message":       "Project added",
				"result.key_id": "*",
				"result.key":    "*",
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID, "keys"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID, "keys"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":          float64(200),
				"message":       "Project added",
				"result.key_id": "*",
				"result.key":    "*",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys") + "?owner_id=" + env.admin.ID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_deleteProjectKey(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys", env.adminProject1API.ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID, "keys", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID, "keys", env.adminProject1API.ID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID, "keys", env.adminProject1API.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project key deleted",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID, "keys", env.userProject3API.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_06",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID, "keys", env.userProject3API.ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project key deleted",
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER:
			Name:       "test2_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID, "keys", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID, "keys", env.adminProject1API.ID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_03",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID, "keys", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, env.userProject13.ID, "keys", env.userProject13API.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project key deleted",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, env.adminProject2.ID, "keys", env.adminProject2API.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_06",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, env.adminProject2.ID, "keys", env.adminProject2API.ID) + "?owner_id=" + env.user.ID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":   float64(404),
				"result": "Member not found",
			},
			RequestHeaders: env.userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}
