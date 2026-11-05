package member

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_schemeMembers(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathMembersScheme,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstFakeID),
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
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                              float64(200),
				"message":                           "Members",
				"result.total":                      float64(1),
				"result.members.0.scheme_member_id": env.adminScheme1User.ID,
				"result.members.0.profile_id":       env.user.ID,
				"result.members.0.name":             "Carly",
				"result.members.0.surname":          "Bender",
				"result.members.0.alias":            "user",
				"result.members.0.active":           true,
				"result.members.0.online":           nil,
				"result.members.0.email":            "user@werbot.net",
				"result.members.0.locked_at":        nil,
				"result.members.0.archived_at":      nil,
				"result.members.0.updated_at":       nil,
				"result.members.0.created_at":       "*",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: search profiles without scheme
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID, "search"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Members without scheme",
				"result.total": float64(6),
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: search profiles without scheme
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID, "search") + "?alias=user1",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                       float64(200),
				"message":                    "Members without scheme",
				"result.total":               float64(1),
				"result.members.0.member_id": env.p1Members["user1"].ID,
				"result.members.0.email":     "user1@werbot.net",
				"result.members.0.online":    true,
				"result.members.0.active":    true,
				"result.members.0.role":      float64(1),
				"result.members.0.alias":     "user1",
				"result.members.0.name":      "Harrison",
				"result.members.0.surname":   "Bowling",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_07",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                              float64(200),
				"message":                           "Members",
				"result.total":                      float64(1),
				"result.members.0.email":            "admin@werbot.net",
				"result.members.0.scheme_member_id": env.userScheme1Admin.ID,
				"result.members.0.profile_id":       env.admin.ID,
				"result.members.0.name":             "Penny",
				"result.members.0.surname":          "Hoyle",
				"result.members.0.alias":            "admin",
				"result.members.0.active":           true,
				"result.members.0.online":           true,
				"result.members.0.locked_at":        nil,
				"result.members.0.archived_at":      nil,
				"result.members.0.updated_at":       "*",
				"result.members.0.created_at":       "*",
				// --
				"result.members.1.scheme_member_id": nil,
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_08",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID, "search") + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                       float64(200),
				"message":                    "Members without scheme",
				"result.total":               float64(11),
				"result.members.0.member_id": env.p3Members["user9"].ID,
				"result.members.0.email":     "user9@werbot.net",
				"result.members.0.online":    true,
				"result.members.0.active":    true,
				"result.members.0.role":      float64(1),
				"result.members.0.alias":     "user9",
				"result.members.0.name":      "Brock",
				"result.members.0.surname":   "Solomon",
				// --
				"result.members.11.scheme_member_id": nil,
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER:
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstFakeID),
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
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                              float64(200),
				"message":                           "Members",
				"result.total":                      float64(1),
				"result.members.0.scheme_member_id": env.userScheme1Admin.ID,
				"result.members.0.profile_id":       nil,
				"result.members.0.name":             "Penny",
				"result.members.0.surname":          "Hoyle",
				"result.members.0.alias":            "admin",
				"result.members.0.active":           true,
				"result.members.0.online":           true,
				"result.members.0.email":            nil,
				"result.members.0.locked_at":        nil,
				"result.members.0.archived_at":      nil,
				"result.members.0.updated_at":       nil,
				"result.members.0.created_at":       "*",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: search profiles without scheme
			Name:       "test2_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID, "search"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Members without scheme",
				"result.total": float64(11),
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: search profiles without scheme
			Name:       "test2_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID, "search") + "?alias=user1",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                       float64(200),
				"message":                    "Members without scheme",
				"result.total":               float64(3),
				"result.members.0.member_id": env.p3Members["user10"].ID,
				"result.members.0.email":     "user10@werbot.net",
				"result.members.0.online":    true,
				"result.members.0.active":    true,
				"result.members.0.role":      float64(1),
				"result.members.0.alias":     "user10",
				"result.members.0.name":      "Clinton",
				"result.members.0.surname":   "Proctor",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_07",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID) + "?owner_id=" + env.admin.ID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_08",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID, "search") + "?owner_id=" + env.admin.ID,
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

func TestHandler_schemeMember(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathMembersScheme,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID, env.adminScheme1User.ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID, test.ConstFakeID),
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
			Path:       test.PathGluing(pathMembersScheme, test.ConstFakeID, env.adminScheme1User.ID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID, env.adminScheme1User.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                    float64(200),
				"message":                 "Member",
				"result.scheme_member_id": env.adminScheme1User.ID,
				"result.profile_id":       env.user.ID,
				"result.name":             "Carly",
				"result.surname":          "Bender",
				"result.alias":            "user",
				"result.active":           true,
				"result.online":           nil,
				"result.email":            "user@werbot.net",
				"result.locked_at":        nil,
				"result.archived_at":      nil,
				"result.updated_at":       nil,
				"result.created_at":       "*",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID, env.adminScheme1User.ID) + "?owner_id=" + test.ConstFakeID,
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
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID, env.userScheme1Admin.ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                    float64(200),
				"message":                 "Member",
				"result.scheme_member_id": env.userScheme1Admin.ID,
				"result.profile_id":       env.admin.ID,
				"result.name":             "Penny",
				"result.surname":          "Hoyle",
				"result.alias":            "admin",
				"result.active":           true,
				"result.online":           true,
				"result.email":            "admin@werbot.net",
				"result.locked_at":        nil,
				"result.archived_at":      nil,
				"result.updated_at":       "*",
				"result.created_at":       "*",
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID, test.ConstFakeID),
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
			Path:       test.PathGluing(pathMembersScheme, test.ConstFakeID, env.userScheme1Admin.ID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID, env.userScheme1Admin.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                    float64(200),
				"message":                 "Member",
				"result.scheme_member_id": env.userScheme1Admin.ID,
				"result.profile_id":       nil,
				"result.name":             "Penny",
				"result.surname":          "Hoyle",
				"result.alias":            "admin",
				"result.active":           true,
				"result.online":           true,
				"result.email":            nil,
				"result.locked_at":        nil,
				"result.archived_at":      nil,
				"result.updated_at":       nil,
				"result.created_at":       "*",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID, env.p3Members["user10"].ID) + "?owner_id=" + test.ConstFakeID,
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
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID, env.p3Members["user10"].ID) + "?owner_id=" + env.admin.ID,
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

func TestHandler_addSchemeMember(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPost,
			Path:       pathMembersScheme,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":             float64(400),
				"message":          "Bad Request",
				"result.member_id": "value is required",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"member_id": "test",
			},
			Body: test.BodyTable{
				"code":             float64(400),
				"message":          "Bad Request",
				"result.member_id": "value must be a valid UUID",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"member_id": test.ConstFakeID,
			},
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
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"member_id": env.p1Members["user1"].ID,
			},
			Body: test.BodyTable{
				"code":                    float64(200),
				"message":                 "Member added",
				"result.scheme_member_id": "*",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: member from other team
			Name:       "test1_05",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"member_id": env.p3Members["user2"].ID,
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
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"member_id": env.p3Members["user2"].ID,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_07",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"member_id": env.p3Members["user11"].ID,
			},
			Body: test.BodyTable{
				"code":                    float64(200),
				"message":                 "Member added",
				"result.scheme_member_id": "*",
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":             float64(400),
				"message":          "Bad Request",
				"result.member_id": "value is required",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"member_id": "test",
			},
			Body: test.BodyTable{
				"code":             float64(400),
				"message":          "Bad Request",
				"result.member_id": "value must be a valid UUID",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"member_id": test.ConstFakeID,
			},
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
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"member_id": env.p3Members["user10"].ID,
			},
			Body: test.BodyTable{
				"code":                    float64(200),
				"message":                 "Member added",
				"result.scheme_member_id": "*",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: member from other team
			Name:       "test2_05",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"member_id": env.p1Members["user2"].ID,
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
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"member_id": env.p1Members["user2"].ID,
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
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID) + "?owner_id=" + env.admin.ID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"member_id": env.p1Members["user2"].ID,
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

func TestHandler_updateSchemeMember(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPatch,
			Path:       pathMembersScheme,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID, env.adminScheme1User.ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID, env.adminScheme1User.ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: turn active member
			Name:       "test1_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID, env.adminScheme1User.ID),
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
		{ // ADMIN: broken scheme UUID
			Name:   "test1_03",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, test.ConstFakeID, env.adminScheme1User.ID),
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: active a member from someone else's project
			Name:   "test1_04",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, env.adminScheme2.ID, env.userScheme2User1.ID),
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			}, RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:   "test1_05",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, env.userScheme2.ID, env.userScheme2User1.ID) + "?owner_id=" + test.ConstFakeID,
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			}, RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: active a member from someone else's project with profile UUID
			Name:   "test1_06",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, env.userScheme2.ID, env.userScheme2User1.ID) + "?owner_id=" + env.user.ID,
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member updated",
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID, env.userScheme1Admin.ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: turn active member
			Name:       "test2_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID, env.userScheme1Admin.ID),
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
		{ // USER: broken scheme UUID
			Name:   "test2_03",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, test.ConstFakeID, env.userScheme1Admin.ID),
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: active a member from someone else's project
			Name:   "test2_04",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, env.userScheme2.ID, env.userScheme1Admin.ID),
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			}, RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:   "test2_05",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, env.userScheme2.ID, env.userScheme1Admin.ID) + "?owner_id=" + test.ConstFakeID,
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: active a member from someone else's project with user UUID
			Name:   "test2_06",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, env.userScheme2.ID, env.userScheme1Admin.ID) + "?owner_id=" + env.user.ID,
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_deleteSchemeMember(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodDelete,
			Path:       pathMembersScheme,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID, test.ConstFakeID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: fake member UUID
			Name:       "test1_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: is not the owner of the scheme
			Name:       "test1_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID, env.userScheme2User1.ID),
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
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme1.ID, env.adminScheme1User.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member deleted",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme2.ID, env.userScheme2User1.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.adminHeader,
		},
		{ // ADMIN: is not the owner of the scheme but can pass the profile_id
			Name:       "test1_05",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme2.ID, env.userScheme2User1.ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member deleted",
			},
			RequestHeaders: env.adminHeader,
		},

		{ // USER: fake member UUID
			Name:       "test2_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: is not the owner of the scheme
			Name:       "test2_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID, env.adminScheme1User.ID),
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
			Path:       test.PathGluing(pathMembersScheme, env.userScheme1.ID, env.userScheme1Admin.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member deleted",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme2.ID, env.adminScheme1User.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.userHeader,
		},
		{ // USER: is not the owner of the scheme but can pass the profile_id
			Name:       "test2_05",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, env.adminScheme2.ID, env.adminScheme1User.ID) + "?owner_id=" + env.admin.ID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: env.userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}
