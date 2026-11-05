package member

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_projectMembers(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathMembersProject,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: broken project UUID
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, "test"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: fake project UUID
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID),
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
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                          float64(200),
				"message":                       "Members",
				"result.total":                  float64(7),
				"result.members.0.member_id":    env.p1Members["user3"].ID,
				"result.members.0.owner_id":     env.admin.ID,
				"result.members.0.owner_name":   "Penny",
				"result.members.0.project_id":   env.project1.ID,
				"result.members.0.project_name": "project1",
				"result.members.0.role":         float64(1),
				"result.members.0.profile_id":   env.profiles["user3"].ID,
				"result.members.0.name":         "Carla",
				"result.members.0.created_at":   "*",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID) + "?owner_id=" + test.ConstFakeID,
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
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Members",
				"result.total": float64(12),
				// -------------------------------
				"result.members.0.active":        true,
				"result.members.0.member_id":     env.p3Members["user1"].ID,
				"result.members.0.online":        true,
				"result.members.0.owner_id":      env.user.ID,
				"result.members.0.owner_name":    "Carly",
				"result.members.0.project_id":    env.project3.ID,
				"result.members.0.project_name":  "project3",
				"result.members.0.role":          float64(1),
				"result.members.0.schemes_count": float64(1),
				"result.members.0.profile_id":    env.profiles["user1"].ID,
				"result.members.0.name":          "Harrison",
				"result.members.0.created_at":    "*",
				// -------------------------------
				"result.members.1.active":        true,
				"result.members.1.member_id":     env.p3Members["admin"].ID,
				"result.members.1.online":        nil,
				"result.members.1.owner_id":      env.user.ID,
				"result.members.1.owner_name":    "Carly",
				"result.members.1.project_id":    env.project3.ID,
				"result.members.1.project_name":  "project3",
				"result.members.1.role":          float64(1),
				"result.members.1.schemes_count": float64(2),
				"result.members.1.profile_id":    env.admin.ID,
				"result.members.1.name":          "Penny",
				"result.members.1.created_at":    "*",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_07",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID) + "?limit=1&owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                           float64(200),
				"message":                        "Members",
				"result.total":                   float64(12),
				"result.members.0.active":        true,
				"result.members.0.member_id":     env.p3Members["user1"].ID,
				"result.members.0.online":        true,
				"result.members.0.owner_id":      env.user.ID,
				"result.members.0.owner_name":    "Carly",
				"result.members.0.project_id":    env.project3.ID,
				"result.members.0.project_name":  "project3",
				"result.members.0.role":          float64(1),
				"result.members.0.schemes_count": float64(1),
				"result.members.0.profile_id":    env.profiles["user1"].ID,
				"result.members.0.name":          "Harrison",
				"result.members.0.created_at":    "*",
				"result.members.1.active":        nil,
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: search profiles without project
			Name:       "test1_08",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, "search"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                         float64(200),
				"message":                      "Profiles without project",
				"result.total":                 float64(8),
				"result.profiles.0.email":      env.admin.Email,
				"result.profiles.0.alias":      "admin",
				"result.profiles.0.profile_id": env.admin.ID,
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: search profiles without project for Profile UUID
			Name:       "test1_09",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID, "search") + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                         float64(200),
				"message":                      "Profiles without project",
				"result.total":                 float64(3),
				"result.profiles.0.email":      "invite201@werbot.net",
				"result.profiles.0.alias":      "invite201",
				"result.profiles.0.profile_id": env.invite201.ID,
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER: fake project UUID
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID),
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
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Members",
				"result.total": float64(11),
				//----------------------------------------
				"result.members.0.owner_id":      nil,
				"result.members.0.project_id":    nil,
				"result.members.0.active":        true,
				"result.members.0.member_id":     env.p3Members["user1"].ID,
				"result.members.0.online":        true,
				"result.members.0.project_name":  "project3",
				"result.members.0.role":          float64(1),
				"result.members.0.schemes_count": float64(1),
				"result.members.0.name":          "Harrison",
				"result.members.0.locked_at":     nil,
				"result.members.0.archived_at":   nil,
				"result.members.0.updated_at":    nil,
				"result.members.0.created_at":    "*",
				//----------------------------------------
				"result.members.1.owner_id":      nil,
				"result.members.1.project_id":    nil,
				"result.members.1.active":        true,
				"result.members.1.member_id":     env.p3Members["admin"].ID,
				"result.members.1.online":        nil,
				"result.members.1.project_name":  "project3",
				"result.members.1.role":          float64(1),
				"result.members.1.schemes_count": float64(2),
				"result.members.1.name":          "Penny",
				"result.members.1.locked_at":     nil,
				"result.members.1.archived_at":   nil,
				"result.members.1.updated_at":    nil,
				"result.members.1.created_at":    "*",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: ignored profile_id
			Name:       "test2_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                        float64(200),
				"message":                     "Members",
				"result.total":                float64(11),
				"result.members.0.member_id":  "*",
				"result.members.1.member_id":  "*",
				"result.members.11.member_id": nil,
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID) + "?owner_id=" + env.admin.ID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: search profiles without project
			Name:       "test2_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID, "search"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: search profiles without project for User UUID
			Name:       "test2_07",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, "search") + "?owner_id=" + env.admin.ID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: env.userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_projectMember(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathMembersProject,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, env.p1Members["user"].ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, env.p1Members["user"].ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                float64(200),
				"message":             "Member",
				"result.member_id":    env.p1Members["user"].ID,
				"result.owner_id":     env.admin.ID,
				"result.owner_name":   "Penny",
				"result.project_id":   env.project1.ID,
				"result.project_name": "project1",
				"result.role":         float64(1),
				"result.profile_id":   env.user.ID,
				"result.name":         "Carly",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID, env.p1Members["user"].ID),
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
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID, test.ConstFakeID),
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
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID, test.ConstFakeID) + "?owner_id=" + env.user.ID,
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
			Path:       test.PathGluing(pathMembersProject, env.project3.ID, env.p3Members["admin"].ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                float64(200),
				"message":             "Member",
				"result.active":       true,
				"result.owner_id":     env.user.ID,
				"result.project_id":   env.project3.ID,
				"result.member_id":    env.p3Members["admin"].ID,
				"result.profile_id":   env.admin.ID,
				"result.owner_name":   "Carly",
				"result.project_name": "project3",
				"result.role":         float64(1),
				"result.name":         "Penny",
				"result.created_at":   "*",
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID, env.p3Members["admin"].ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                float64(200),
				"message":             "Member",
				"result.member_id":    env.p3Members["admin"].ID,
				"result.project_name": "project3",
				"result.role":         float64(1),
				"result.name":         "Penny",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID, env.p1Members["user"].ID),
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
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID, test.ConstFakeID),
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
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID, test.ConstFakeID) + "?owner_id=" + env.user.ID,
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
			Path:       test.PathGluing(pathMembersProject, env.project3.ID, env.p1Members["user"].ID) + "?owner_id=" + env.user.ID,
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

func TestHandler_addProjectMember(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPost,
			Path:       pathMembersProject,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":              float64(400),
				"message":           "Bad Request",
				"result.profile_id": "value is required",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"profile_id": "test",
			},
			Body: test.BodyTable{
				"code":              float64(400),
				"message":           "Bad Request",
				"result.profile_id": "value must be a valid UUID",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"profile_id": env.user20.ID,
			},
			Body: test.BodyTable{
				"code":             float64(200),
				"message":          "Member added",
				"result.member_id": "*",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"profile_id": env.user20.ID,
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
			Path:       test.PathGluing(pathMembersProject, env.project1.ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"profile_id": test.ConstFakeID,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_06",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"profile_id": env.user20.ID,
			},
			Body: test.BodyTable{
				"code":             float64(200),
				"message":          "Member added",
				"result.member_id": "*",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_07",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"profile_id": env.user20.ID,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":              float64(400),
				"message":           "Bad Request",
				"result.profile_id": "value is required",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"profile_id": "test",
			},
			Body: test.BodyTable{
				"code":              float64(400),
				"message":           "Bad Request",
				"result.profile_id": "value must be a valid UUID",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "tes2_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"profile_id": env.user20.ID,
			},
			Body: test.BodyTable{
				"code":             float64(200),
				"message":          "Member added",
				"result.member_id": "*",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"profile_id": env.user20.ID,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"profile_id": test.ConstFakeID,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_06",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID) + "?owner_id=" + env.user.ID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"profile_id": env.user20.ID,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_07",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"profile_id": env.user20.ID,
			},
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

func TestHandler_updateProjectMember(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPatch,
			Path:       pathMembersProject,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, test.ConstFakeID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: broken member UUID
			Name:       "test1_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, test.ConstFakeID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: broken member UUID
			Name:       "test1_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, test.ConstFakeID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"active": true,
			},
			Body:           test.BodyNotFound,
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: update a member active
			Name:       "test1_03",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, env.p1Members["user"].ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"active": true,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member updated",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: update a member active
			Name:       "test1_04",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, env.p1Members["user"].ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"role": 1,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member updated",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID, env.p3Members["admin"].ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"role": 1,
			},
			Body:           test.BodyNotFound,
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_06",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID, env.p3Members["admin"].ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"role": 2,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member updated",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_07",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID, env.p3Members["admin"].ID) + "?owner_id=" + env.user.ID,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"role": 3,
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER: broken member UUID
			Name:       "test2_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID, test.ConstFakeID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: broken member UUID
			Name:       "test2_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID, test.ConstFakeID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"active": true,
			},
			Body:           test.BodyNotFound,
			RequestHeaders: env.userHeader,
		},
		{ // USER: update a member active
			Name:       "test2_03",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID, env.p3Members["admin"].ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"active": true,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member updated",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: update a member active
			Name:       "test2_04",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID, env.p3Members["admin"].ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"role": 1,
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, env.p1Members["user"].ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"role": 1,
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
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, env.p1Members["user"].ID) + "?owner_id=" + env.admin.ID,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"role": 1,
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_07",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, env.p1Members["user"].ID) + "?owner_id=" + env.admin.ID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"active": true,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: env.userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_removeProjectMember(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodDelete,
			Path:       pathMembersProject,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, env.p1Members["user"].ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersProject, env.project1.ID, env.p1Members["user"].ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member deleted",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:           "test1_02",
			Method:         http.MethodDelete,
			Path:           test.PathGluing(pathMembersProject, env.project1.ID, test.ConstFakeID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:           "test1_03",
			Method:         http.MethodDelete,
			Path:           test.PathGluing(pathMembersProject, env.project1.ID, env.p1Members["user"].ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID, env.p3Members["admin"].ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member deleted",
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersProject, env.project3.ID, env.p3Members["user1"].ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member deleted",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:           "test2_02",
			Method:         http.MethodDelete,
			Path:           test.PathGluing(pathMembersProject, env.project3.ID, test.ConstFakeID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:           "test2_03",
			Method:         http.MethodDelete,
			Path:           test.PathGluing(pathMembersProject, env.project3.ID, env.p1Members["user"].ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:           "test2_04",
			Method:         http.MethodDelete,
			Path:           test.PathGluing(pathMembersProject, env.project1.ID, env.p1Members["user"].ID) + "?owner_id=" + env.admin.ID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: env.userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}
