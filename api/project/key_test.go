package project

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/pkg/crypto"
)

func TestHandler_projectKeys(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys"),
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
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                      float64(200),
				"message":                   "Project keys",
				"result.total":              float64(1),
				"result.keys.0.key_id":      "a1059d92-d032-427c-9444-571967d1f9a5",
				"result.keys.0.online":      true,
				"result.keys.0.key":         "3GZBSPqDi7r1FDzYMUNV41l9HOJlb9y8b3ZI9",
				"result.keys.0.secret":      "AweHj3rtANGfy0gG021ptsDzYMwYmgwnY11CC",
				"result.keys.0.locked_at":   nil,
				"result.keys.0.archived_at": nil,
				"result.keys.0.updated_at":  nil,
				"result.keys.0.created_at":  "*",
				// --
				"result.keys.1.key_id": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID, "keys") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID, "keys") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                      float64(200),
				"message":                   "Project keys",
				"result.total":              float64(1),
				"result.keys.0.key_id":      "268def26-4efb-47c4-b699-f34903cf05f5",
				"result.keys.0.key":         "5tYJOkr3oLCOEvhw3nB83AmDzYM7yJsJ0Sonl",
				"result.keys.0.secret":      "aDzYMy9g3mmsq3XazPLvvCbj4kJAsgatxBDVW",
				"result.keys.0.online":      true,
				"result.keys.0.locked_at":   nil,
				"result.keys.0.archived_at": nil,
				"result.keys.0.updated_at":  nil,
				"result.keys.0.created_at":  "*",
				// --
				"result.keys.1.key_id": nil,
			},
			RequestHeaders: adminHeader,
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
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID, "keys"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                      float64(200),
				"message":                   "Project keys",
				"result.total":              float64(1),
				"result.keys.0.key_id":      "268def26-4efb-47c4-b699-f34903cf05f5",
				"result.keys.0.key":         "5tYJOkr3oLCOEvhw3nB83AmDzYM7yJsJ0Sonl",
				"result.keys.0.secret":      "aDzYMy9g3mmsq3XazPLvvCbj4kJAsgatxBDVW",
				"result.keys.0.online":      true,
				"result.keys.0.locked_at":   nil,
				"result.keys.0.archived_at": nil,
				"result.keys.0.updated_at":  nil,
				"result.keys.0.created_at":  "*",
				// --
				"result.keys.1.key_id": nil,
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys") + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_projectKey(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
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
			Path:       test.PathGluing(pathProjects, "key", "3GZBSPqDi7r1FDzYMUNV41l9HOJlb9y8b3ZI9"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":              float64(200),
				"message":           "Project key",
				"result.project_id": "2bef1080-cd6e-49e5-8042-1224cf6a3da9",
			},
		},

		// private section
		{ // unauthorized request
			Name:       "test0_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys", test.ConstAdminProject1ApiID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},

		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID, "keys", test.ConstAdminProject1ApiID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:           "test1_03",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathProjects, test.ConstAdminProject1ID, "test", test.ConstAdminProject1ApiID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys", test.ConstAdminProject1ApiID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Project key",
				"result.key":         "3GZBSPqDi7r1FDzYMUNV41l9HOJlb9y8b3ZI9",
				"result.secret":      "AweHj3rtANGfy0gG021ptsDzYMwYmgwnY11CC",
				"result.online":      true,
				"result.locked_at":   nil,
				"result.archived_at": nil,
				"result.updated_at":  nil,
				"result.created_at":  "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID, "keys", "268def26-4efb-47c4-b699-f34903cf05f5"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID, "keys", "268def26-4efb-47c4-b699-f34903cf05f5") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_07",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID, "keys", "268def26-4efb-47c4-b699-f34903cf05f5") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Project key",
				"result.key":         "5tYJOkr3oLCOEvhw3nB83AmDzYM7yJsJ0Sonl",
				"result.secret":      "aDzYMy9g3mmsq3XazPLvvCbj4kJAsgatxBDVW",
				"result.online":      true,
				"result.locked_at":   nil,
				"result.archived_at": nil,
				"result.updated_at":  nil,
				"result.created_at":  "*",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER:
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID, "keys", "268def26-4efb-47c4-b699-f34903cf05f5"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID, "keys", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:           "test2_03",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathProjects, test.ConstUserProject1ID, "test", "268def26-4efb-47c4-b699-f34903cf05f5"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID, "keys", "268def26-4efb-47c4-b699-f34903cf05f5"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Project key",
				"result.key":         "5tYJOkr3oLCOEvhw3nB83AmDzYM7yJsJ0Sonl",
				"result.secret":      "aDzYMy9g3mmsq3XazPLvvCbj4kJAsgatxBDVW",
				"result.online":      true,
				"result.locked_at":   nil,
				"result.archived_at": nil,
				"result.updated_at":  nil,
				"result.created_at":  "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys", test.ConstAdminProject1ApiID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys", test.ConstAdminProject1ApiID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_07",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys", test.ConstAdminProject1ApiID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_addProjectKey(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys"),
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
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":          float64(200),
				"message":       "Project added",
				"result.key_id": "*",
				"result.key":    "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID, "keys") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID, "keys") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":          float64(200),
				"message":       "Project added",
				"result.key_id": "*",
				"result.key":    "*",
			},
			RequestHeaders: adminHeader,
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
			RequestHeaders: userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID, "keys"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":          float64(200),
				"message":       "Project added",
				"result.key_id": "*",
				"result.key":    "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys") + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_deleteProjectKey(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys", test.ConstAdminProject1ApiID),
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
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID, "keys", test.ConstAdminProject1ApiID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID, "keys", test.ConstAdminProject1ApiID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project key deleted",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID, "keys", "268def26-4efb-47c4-b699-f34903cf05f5") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_06",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID, "keys", "268def26-4efb-47c4-b699-f34903cf05f5") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project key deleted",
			},
			RequestHeaders: adminHeader,
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
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID, "keys", test.ConstAdminProject1ApiID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_03",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID, "keys", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, "f85c4597-005d-48a6-b643-a21adf19a4aa", "keys", "81ae0bbe-b746-44a6-a1f4-8467ff17bf5e"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project key deleted",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, "e29568e6-c8f8-4555-a531-58a44554046f", "keys", "974c8001-4f8a-4ccc-b9db-9cf29e1405b2") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_06",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, "e29568e6-c8f8-4555-a531-58a44554046f", "keys", "974c8001-4f8a-4ccc-b9db-9cf29e1405b2") + "?owner_id=" + test.ConstUserID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":   float64(404),
				"result": "Member not found",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}
