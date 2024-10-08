package project

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_projects(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathProjects,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request with user UUID
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       pathProjects + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Projects",
				"result.total": float64(3),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with fake user UUID
			Name:           "test1_02",
			Method:         http.MethodGet,
			Path:           pathProjects + "?owner_id=" + test.ConstFakeID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       pathProjects,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                                 float64(200),
				"message":                              "Projects",
				"result.total":                         float64(11),
				"result.projects.0.alias":              "68rwRW",
				"result.projects.0.owner_id":           "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				"result.projects.0.project_id":         "26060c68-5a06-4a57-b87a-be0f1e787157",
				"result.projects.0.title":              "project7",
				"result.projects.0.servers_count":      nil,
				"result.projects.0.databases_count":    nil,
				"result.projects.0.applications_count": nil,
				"result.projects.0.desktops_count":     nil,
				"result.projects.0.containers_count":   nil,
				"result.projects.0.clouds_count":       nil,
				"result.projects.0.locked_at":          nil,
				"result.projects.0.archived_at":        nil,
				"result.projects.0.updated_at":         nil,
				"result.projects.0.created_at":         "*",
				// --
				"result.projects.11.alias": nil,
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       pathProjects,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Projects",
				"result.total": float64(3),
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with user UUID
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       pathProjects + "?owner_id=" + test.ConstAdminID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                                 float64(200),
				"message":                              "Projects",
				"result.total":                         float64(3),
				"result.projects.0.alias":              "9Yyz9Z",
				"result.projects.0.owner_id":           nil,
				"result.projects.0.project_id":         "83d401e4-fda4-404e-8c2a-da58b03919c1",
				"result.projects.0.title":              "project13",
				"result.projects.0.servers_count":      nil,
				"result.projects.0.databases_count":    nil,
				"result.projects.0.applications_count": nil,
				"result.projects.0.desktops_count":     nil,
				"result.projects.0.containers_count":   nil,
				"result.projects.0.clouds_count":       nil,
				"result.projects.0.locked_at":          nil,
				"result.projects.0.archived_at":        nil,
				"result.projects.0.updated_at":         nil,
				"result.projects.0.created_at":         "*",
				// --
				"result.projects.3.alias": nil,
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_project(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
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
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID),
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
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID),
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
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID) + "?owner_id=" + test.ConstUserID,
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
			RequestHeaders: adminHeader,
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
			RequestHeaders: userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID),
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
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test3_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID),
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
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstAdminID,
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

func TestHandler_addProject(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
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
			RequestHeaders: adminHeader,
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
			RequestHeaders: adminHeader,
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
			RequestHeaders: adminHeader,
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
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodPost,
			Path:       pathProjects + "?owner_id=" + test.ConstUserID,
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
			RequestHeaders: adminHeader,
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
			RequestHeaders: userHeader,
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
			RequestHeaders: userHeader,
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
			RequestHeaders: userHeader,
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
			RequestHeaders: userHeader,
		},
		{ // USER: ignored owner_id
			Name:       "test2_05",
			Method:     http.MethodPost,
			Path:       pathProjects + "?owner_id=" + test.ConstAdminID,
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
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_updateProject(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
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
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"alias": "ABC",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.alias": "value does not match regex pattern `^[a-z0-9]+$`",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_03",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title": "12",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.title": "value length must be at least 3 characters",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"alias": "a1s2d3",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project updated",
				"result":  nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_06",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project updated",
				"result":  nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_07",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_08",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_09",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project updated",
				"result":  nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_10",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"alias": "q1w2e3",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project updated",
				"result":  nil,
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"alias": "ABC",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.alias": "value does not match regex pattern `^[a-z0-9]+$`",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_03",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title": "12",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.title": "value length must be at least 3 characters",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"alias": "a1s2d3",
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_06",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project updated",
				"result":  nil,
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_07",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_08",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_09",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"title": "title",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_10",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"alias": "q1w2e3",
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_deleteProject(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
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
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project deleted",
				"result":  nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID) + "?owner_id=" + test.ConstFakeID,
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
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project deleted",
				"result":  nil,
			},
			RequestHeaders: adminHeader,
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
			RequestHeaders: userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstUserProject1ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Project deleted",
				"result":  nil,
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_03",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstFakeID,
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
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathProjects, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstAdminID,
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
