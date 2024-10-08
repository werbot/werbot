package member

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_schemeMembers(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
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
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID),
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
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                              float64(200),
				"message":                           "Members",
				"result.total":                      float64(1),
				"result.members.0.scheme_member_id": "8ca0ad93-4338-44cb-93dd-3f57272d0ffa",
				"result.members.0.user_id":          "c180ad5c-0c65-4cee-8725-12931cb5abb3",
				"result.members.0.user_name":        "Carly",
				"result.members.0.user_surname":     "Bender",
				"result.members.0.user_alias":       "user",
				"result.members.0.active":           true,
				"result.members.0.online":           nil,
				"result.members.0.email":            "user@werbot.net",
				"result.members.0.locked_at":        nil,
				"result.members.0.archived_at":      nil,
				"result.members.0.updated_at":       nil,
				"result.members.0.created_at":       "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: search users without scheme
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID, "search"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Members without scheme",
				"result.total": float64(6),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: search users without scheme
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID, "search") + "?alias=user1",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                          float64(200),
				"message":                       "Members without scheme",
				"result.total":                  float64(1),
				"result.members.0.member_id":    "92de7a44-08fc-4d42-aab5-37f86fd598a2",
				"result.members.0.email":        "user1@werbot.net",
				"result.members.0.online":       true,
				"result.members.0.active":       true,
				"result.members.0.role":         float64(1),
				"result.members.0.user_alias":   "user1",
				"result.members.0.user_name":    "Harrison",
				"result.members.0.user_surname": "Bowling",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_07",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                              float64(200),
				"message":                           "Members",
				"result.total":                      float64(1),
				"result.members.0.email":            "admin@werbot.net",
				"result.members.0.scheme_member_id": "57ea9d56-5382-4749-99bb-b71a38d448b0",
				"result.members.0.user_id":          "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				"result.members.0.user_name":        "Penny",
				"result.members.0.user_surname":     "Hoyle",
				"result.members.0.user_alias":       "admin",
				"result.members.0.active":           true,
				"result.members.0.online":           true,
				"result.members.0.locked_at":        nil,
				"result.members.0.archived_at":      nil,
				"result.members.0.updated_at":       "*",
				"result.members.0.created_at":       "*",
				// --
				"result.members.1.scheme_member_id": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_08",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID, "search") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                          float64(200),
				"message":                       "Members without scheme",
				"result.total":                  float64(11),
				"result.members.0.member_id":    "7f717b66-34b5-4707-b9b8-0f63e8e034de",
				"result.members.0.email":        "user9@werbot.net",
				"result.members.0.online":       true,
				"result.members.0.active":       true,
				"result.members.0.role":         float64(1),
				"result.members.0.user_alias":   "user9",
				"result.members.0.user_name":    "Brock",
				"result.members.0.user_surname": "Solomon",
				// --
				"result.members.11.scheme_member_id": nil,
			},
			RequestHeaders: adminHeader,
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
			RequestHeaders: userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                              float64(200),
				"message":                           "Members",
				"result.total":                      float64(1),
				"result.members.0.scheme_member_id": "57ea9d56-5382-4749-99bb-b71a38d448b0",
				"result.members.0.user_id":          nil,
				"result.members.0.user_name":        "Penny",
				"result.members.0.user_surname":     "Hoyle",
				"result.members.0.user_alias":       "admin",
				"result.members.0.active":           true,
				"result.members.0.online":           true,
				"result.members.0.email":            nil,
				"result.members.0.locked_at":        nil,
				"result.members.0.archived_at":      nil,
				"result.members.0.updated_at":       nil,
				"result.members.0.created_at":       "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: search users without scheme
			Name:       "test2_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID, "search"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Members without scheme",
				"result.total": float64(11),
			},
			RequestHeaders: userHeader,
		},
		{ // USER: search users without scheme
			Name:       "test2_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID, "search") + "?alias=user1",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                          float64(200),
				"message":                       "Members without scheme",
				"result.total":                  float64(3),
				"result.members.0.member_id":    "de040931-6977-4629-aab0-6d621ff368a3",
				"result.members.0.email":        "user10@werbot.net",
				"result.members.0.online":       true,
				"result.members.0.active":       true,
				"result.members.0.role":         float64(1),
				"result.members.0.user_alias":   "user10",
				"result.members.0.user_name":    "Clinton",
				"result.members.0.user_surname": "Proctor",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_07",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Project not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_08",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID, "search") + "?owner_id=" + test.ConstAdminID,
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

func TestHandler_schemeMember(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
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
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID, test.ConstUserScheme1MemberID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID, test.ConstFakeID),
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
			Path:       test.PathGluing(pathMembersScheme, test.ConstFakeID, test.ConstUserScheme1MemberID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID, test.ConstUserScheme1MemberID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                    float64(200),
				"message":                 "Member",
				"result.scheme_member_id": "8ca0ad93-4338-44cb-93dd-3f57272d0ffa",
				"result.user_id":          "c180ad5c-0c65-4cee-8725-12931cb5abb3",
				"result.user_name":        "Carly",
				"result.user_surname":     "Bender",
				"result.user_alias":       "user",
				"result.active":           true,
				"result.online":           nil,
				"result.email":            "user@werbot.net",
				"result.locked_at":        nil,
				"result.archived_at":      nil,
				"result.updated_at":       nil,
				"result.created_at":       "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID, test.ConstUserScheme1MemberID) + "?owner_id=" + test.ConstFakeID,
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
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID, "57ea9d56-5382-4749-99bb-b71a38d448b0") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                    float64(200),
				"message":                 "Member",
				"result.scheme_member_id": "57ea9d56-5382-4749-99bb-b71a38d448b0",
				"result.user_id":          "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				"result.user_name":        "Penny",
				"result.user_surname":     "Hoyle",
				"result.user_alias":       "admin",
				"result.active":           true,
				"result.online":           true,
				"result.email":            "admin@werbot.net",
				"result.locked_at":        nil,
				"result.archived_at":      nil,
				"result.updated_at":       "*",
				"result.created_at":       "*",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID, test.ConstFakeID),
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
			Path:       test.PathGluing(pathMembersScheme, test.ConstFakeID, "57ea9d56-5382-4749-99bb-b71a38d448b0"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID, "57ea9d56-5382-4749-99bb-b71a38d448b0"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                    float64(200),
				"message":                 "Member",
				"result.scheme_member_id": "57ea9d56-5382-4749-99bb-b71a38d448b0",
				"result.user_id":          nil,
				"result.user_name":        "Penny",
				"result.user_surname":     "Hoyle",
				"result.user_alias":       "admin",
				"result.active":           true,
				"result.online":           true,
				"result.email":            nil,
				"result.locked_at":        nil,
				"result.archived_at":      nil,
				"result.updated_at":       nil,
				"result.created_at":       "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID, "de040931-6977-4629-aab0-6d621ff368a3") + "?owner_id=" + test.ConstFakeID,
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
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID, "de040931-6977-4629-aab0-6d621ff368a3") + "?owner_id=" + test.ConstAdminID,
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

func TestHandler_addSchemeMember(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
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
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":             float64(400),
				"message":          "Bad Request",
				"result.member_id": "value is required",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"member_id": "test",
			},
			Body: test.BodyTable{
				"code":             float64(400),
				"message":          "Bad Request",
				"result.member_id": "value must be a valid UUID",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"member_id": test.ConstFakeID,
			},
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
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"member_id": "92de7a44-08fc-4d42-aab5-37f86fd598a2",
			},
			Body: test.BodyTable{
				"code":                    float64(200),
				"message":                 "Member added",
				"result.scheme_member_id": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: member from other team
			Name:       "test1_05",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"member_id": "43ab80dd-ffe6-4881-aa8d-52b56ea715d2",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_06",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"member_id": "43ab80dd-ffe6-4881-aa8d-52b56ea715d2",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_07",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"member_id": "e7492145-3b8e-4d06-a3a7-81fb6deb3571",
			},
			Body: test.BodyTable{
				"code":                    float64(200),
				"message":                 "Member added",
				"result.scheme_member_id": "*",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":             float64(400),
				"message":          "Bad Request",
				"result.member_id": "value is required",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"member_id": "test",
			},
			Body: test.BodyTable{
				"code":             float64(400),
				"message":          "Bad Request",
				"result.member_id": "value must be a valid UUID",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"member_id": test.ConstFakeID,
			},
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
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"member_id": "de040931-6977-4629-aab0-6d621ff368a3",
			},
			Body: test.BodyTable{
				"code":                    float64(200),
				"message":                 "Member added",
				"result.scheme_member_id": "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: member from other team
			Name:       "test2_05",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"member_id": "bac61932-ac1d-4f3a-b842-17b136bd1346",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_06",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"member_id": "bac61932-ac1d-4f3a-b842-17b136bd1346",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_07",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"member_id": "bac61932-ac1d-4f3a-b842-17b136bd1346",
			},
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

func TestHandler_updateSchemeMember(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
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
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID, test.ConstUserScheme1MemberID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID, test.ConstUserScheme1MemberID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: turn active member
			Name:       "test1_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID, test.ConstUserScheme1MemberID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"active": true,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: broken scheme UUID
			Name:   "test1_03",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, test.ConstFakeID, test.ConstUserScheme1MemberID),
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: active a member from someone else's project
			Name:   "test1_04",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH2ID, test.ConstUserScheme2MemberID),
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			}, RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:   "test1_05",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH2ID, test.ConstUserScheme2MemberID) + "?owner_id=" + test.ConstFakeID,
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			}, RequestHeaders: adminHeader,
		},
		{ // ADMIN: active a member from someone else's project with user UUID
			Name:   "test1_06",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH2ID, test.ConstUserScheme2MemberID) + "?owner_id=" + test.ConstUserID,
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member updated",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID, "57ea9d56-5382-4749-99bb-b71a38d448b0"),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: turn active member
			Name:       "test2_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID, "57ea9d56-5382-4749-99bb-b71a38d448b0"),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"active": true,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member updated",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: broken scheme UUID
			Name:   "test2_03",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, test.ConstFakeID, "57ea9d56-5382-4749-99bb-b71a38d448b0"),
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: active a member from someone else's project
			Name:   "test2_04",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH2ID, "57ea9d56-5382-4749-99bb-b71a38d448b0"),
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			}, RequestHeaders: userHeader,
		},
		{ // USER:
			Name:   "test2_05",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH2ID, "57ea9d56-5382-4749-99bb-b71a38d448b0") + "?owner_id=" + test.ConstFakeID,
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: active a member from someone else's project with user UUID
			Name:   "test2_06",
			Method: http.MethodPatch,
			Path:   test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH2ID, "57ea9d56-5382-4749-99bb-b71a38d448b0") + "?owner_id=" + test.ConstUserID,
			RequestBody: test.BodyTable{
				"active": true,
			},
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_deleteSchemeMember(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
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
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID, test.ConstFakeID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: fake member UUID
			Name:       "test1_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: is not the owner of the scheme
			Name:       "test1_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID, test.ConstUserScheme2MemberID),
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
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH1ID, "8ca0ad93-4338-44cb-93dd-3f57272d0ffa"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member deleted",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH2ID, test.ConstUserScheme2MemberID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: is not the owner of the scheme but can pass the user_id
			Name:       "test1_05",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH2ID, test.ConstUserScheme2MemberID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member deleted",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: fake member UUID
			Name:       "test2_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: is not the owner of the scheme
			Name:       "test2_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID, "8ca0ad93-4338-44cb-93dd-3f57272d0ffa"),
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
			Path:       test.PathGluing(pathMembersScheme, test.ConstUserSchemeSSH1ID, "57ea9d56-5382-4749-99bb-b71a38d448b0"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member deleted",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH2ID, "8ca0ad93-4338-44cb-93dd-3f57272d0ffa") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: is not the owner of the scheme but can pass the user_id
			Name:       "test2_05",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersScheme, test.ConstAdminSchemeSSH2ID, "8ca0ad93-4338-44cb-93dd-3f57272d0ffa") + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}
