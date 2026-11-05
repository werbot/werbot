package project

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_projects(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathProjects,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request with profile UUID
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       pathProjects + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Projects",
				"result.total": float64(2),
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: request with fake profile UUID
			Name:           "test1_02",
			Method:         http.MethodGet,
			Path:           pathProjects + "?owner_id=" + test.ConstFakeID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       pathProjects,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Projects",
				"result.total": float64(3),
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       pathProjects,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Projects",
				"result.total": float64(2),
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: request with profile UUID
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       pathProjects + "?owner_id=" + env.admin.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Projects",
				"result.total": float64(2),
			},
			RequestHeaders: env.userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)

	assertProjectInList(t, app, pathProjects, env.adminHeader, projectListItem{
		ProjectID: env.adminProject7.ID,
		OwnerID:   env.admin.ID,
		Title:     "project7",
		Alias:     "68rwRW",
	})
	// A non-admin listing is ghosted, so owner_id is absent.
	assertProjectInList(t, app, pathProjects+"?owner_id="+env.admin.ID, env.userHeader, projectListItem{
		ProjectID: env.userProject13.ID,
		Title:     "project13",
		Alias:     "9Yyz9Z",
	})
}

func TestHandler_project(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathProjects,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID),
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
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                      float64(200),
				"message":                   "Project",
				"result.alias":              "Y93iyI",
				"result.title":              "project1",
				"result.servers_count":      nil,
				"result.databases_count":    nil,
				"result.applications_count": nil,
				"result.desktops_count":     nil,
				"result.containers_count":   nil,
				"result.clouds_count":       nil,
				"result.locked_at":          nil,
				"result.archived_at":        nil,
				"result.updated_at":         nil,
				"result.created_at":         "*",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID),
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
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                      float64(200),
				"message":                   "Project",
				"result.alias":              "C8cXx0",
				"result.title":              "project3",
				"result.servers_count":      nil,
				"result.databases_count":    nil,
				"result.applications_count": nil,
				"result.desktops_count":     nil,
				"result.containers_count":   nil,
				"result.clouds_count":       nil,
				"result.locked_at":          nil,
				"result.archived_at":        nil,
				"result.updated_at":         nil,
				"result.created_at":         "*",
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER:
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID),
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
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                      float64(200),
				"message":                   "Project",
				"result.alias":              "C8cXx0",
				"result.title":              "project3",
				"result.servers_count":      nil,
				"result.databases_count":    nil,
				"result.applications_count": nil,
				"result.desktops_count":     nil,
				"result.containers_count":   nil,
				"result.clouds_count":       nil,
				"result.locked_at":          nil,
				"result.archived_at":        nil,
				"result.updated_at":         nil,
				"result.created_at":         "*",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test3_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID),
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
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID) + "?owner_id=" + env.admin.ID,
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

func TestHandler_addProject(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPost,
			Path:       pathProjects,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodPost,
			Path:       pathProjects,
			StatusCode: 400,
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.alias": "value is required",
				"result.title": "value is required",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_02",
			Method:     http.MethodPost,
			Path:       pathProjects,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"alias": "ABC",
				"title": "12",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.alias": "value does not match regex pattern `^[a-z0-9]+$`",
				"result.title": "value length must be at least 3 characters",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodPost,
			Path:       pathProjects,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"alias": "a1b2c3",
				"title": "Title",
			},
			Body: test.BodyTable{
				"code":              float64(200),
				"message":           "Project added",
				"result.project_id": "*",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodPost,
			Path:       pathProjects + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"alias": "a1b2c3",
				"title": "Title",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodPost,
			Path:       pathProjects + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"alias": "a1b2c3",
				"title": "Title",
			},
			Body: test.BodyTable{
				"code":              float64(200),
				"message":           "Project added",
				"result.project_id": "*",
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodPost,
			Path:       pathProjects,
			StatusCode: 400,
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.alias": "value is required",
				"result.title": "value is required",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_02",
			Method:     http.MethodPost,
			Path:       pathProjects,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"alias": "ABC",
				"title": "12",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.alias": "value does not match regex pattern `^[a-z0-9]+$`",
				"result.title": "value length must be at least 3 characters",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_03",
			Method:     http.MethodPost,
			Path:       pathProjects,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"alias": "a1b2c3",
				"title": "Title",
			},
			Body: test.BodyTable{
				"code":              float64(200),
				"message":           "Project added",
				"result.project_id": "*",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: ignored owner_id
			Name:       "test2_04",
			Method:     http.MethodPost,
			Path:       pathProjects + "?owner_id=" + test.ConstFakeID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"alias": "a1b2c3",
				"title": "Title",
			},
			Body: test.BodyTable{
				"code":              float64(200),
				"message":           "Project added",
				"result.project_id": "*",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: ignored owner_id
			Name:       "test2_05",
			Method:     http.MethodPost,
			Path:       pathProjects + "?owner_id=" + env.admin.ID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"alias": "a1b2c3",
				"title": "Title",
			},
			Body: test.BodyTable{
				"code":              float64(200),
				"message":           "Project added",
				"result.project_id": "*",
			},
			RequestHeaders: env.userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_updateProject(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPatch,
			Path:       pathProjects,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"alias": "ABC",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.alias": "value does not match regex pattern `^[a-z0-9]+$`",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_03",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title": "12",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.title": "value length must be at least 3 characters",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"alias": "a1s2d3",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project updated",
				"result":  nil,
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_06",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project updated",
				"result":  nil,
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_07",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_08",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_09",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project updated",
				"result":  nil,
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_10",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"alias": "q1w2e3",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project updated",
				"result":  nil,
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"alias": "ABC",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.alias": "value does not match regex pattern `^[a-z0-9]+$`",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_03",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title": "12",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.title": "value length must be at least 3 characters",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"alias": "a1s2d3",
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_06",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project updated",
				"result":  nil,
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_07",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_08",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_09",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID) + "?owner_id=" + env.admin.ID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_10",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID) + "?owner_id=" + env.admin.ID,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"alias": "q1w2e3",
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: env.userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_deleteProject(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodDelete,
			Path:       pathProjects,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project deleted",
				"result":  nil,
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID) + "?owner_id=" + test.ConstFakeID,
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
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project deleted",
				"result":  nil,
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER:
			Name:       "test2_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, env.userProject3.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project deleted",
				"result":  nil,
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_03",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID) + "?owner_id=" + test.ConstFakeID,
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
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, env.adminProject1.ID) + "?owner_id=" + env.admin.ID,
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
