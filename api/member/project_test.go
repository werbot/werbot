package member

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_projectMembers(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
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
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID),
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
			RequestHeaders: adminHeader,
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
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                          float64(200),
				"message":                       "Members",
				"result.total":                  float64(7),
				"result.members.0.member_id":    "455b8913-c71d-4536-9b03-70bcb487b7cb",
				"result.members.0.owner_id":     test.ConstAdminID,
				"result.members.0.owner_name":   "Penny",
				"result.members.0.project_id":   test.ConstAdminProject1ID,
				"result.members.0.project_name": "project1",
				"result.members.0.role":         float64(1),
				"result.members.0.profile_id":   "68bf07a3-0132-4709-920b-5054f9eaa89a",
				"result.members.0.name":         "Carla",
				//"result.members.0.updated_at":   "*",
				"result.members.0.created_at": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID) + "?owner_id=" + test.ConstFakeID,
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
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Members",
				"result.total": float64(12),
				// -------------------------------
				"result.members.0.active":        true,
				"result.members.0.member_id":     "4fc69519-b683-46f0-860c-3e7f12a17563",
				"result.members.0.online":        nil,
				"result.members.0.owner_id":      test.ConstUserID,
				"result.members.0.owner_name":    "Carly",
				"result.members.0.project_id":    test.ConstUserProject1ID,
				"result.members.0.project_name":  "project3",
				"result.members.0.role":          float64(1),
				"result.members.0.schemes_count": float64(16),
				"result.members.0.profile_id":    "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				"result.members.0.name":          "Penny",
				"result.members.0.created_at":    "*",
				// -------------------------------
				"result.members.1.active":        true,
				"result.members.1.member_id":     "49a10a09-0bb3-48af-99cb-181533692585",
				"result.members.1.owner_id":      test.ConstUserID,
				"result.members.1.owner_name":    "Carly",
				"result.members.1.project_id":    test.ConstUserProject1ID,
				"result.members.1.project_name":  "project3",
				"result.members.1.role":          float64(1),
				"result.members.1.schemes_count": float64(1),
				"result.members.1.profile_id":    "b3dc36e2-7f84-414b-b147-7ac850369518",
				"result.members.1.name":          "Harrison",
				"result.members.1.created_at":    "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_07",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID) + "?limit=1&owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                           float64(200),
				"message":                        "Members",
				"result.total":                   float64(12),
				"result.members.0.active":        true,
				"result.members.0.member_id":     "49a10a09-0bb3-48af-99cb-181533692585",
				"result.members.0.online":        true,
				"result.members.0.owner_id":      test.ConstUserID,
				"result.members.0.owner_name":    "Carly",
				"result.members.0.project_id":    test.ConstUserProject1ID,
				"result.members.0.project_name":  "project3",
				"result.members.0.role":          float64(1),
				"result.members.0.schemes_count": float64(1),
				"result.members.0.profile_id":    "b3dc36e2-7f84-414b-b147-7ac850369518",
				"result.members.0.name":          "Harrison",
				"result.members.0.created_at":    "*",
				"result.members.1.active":        nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: search profiles without project
			Name:       "test1_08",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, "search"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                         float64(200),
				"message":                      "Profiles without project",
				"result.total":                 float64(15),
				"result.profiles.0.email":      test.ConstAdminEmail,
				"result.profiles.0.alias":      "admin",
				"result.profiles.0.profile_id": test.ConstAdminID,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: search profiles without project for Profile UUID
			Name:       "test1_09",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID, "search") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                         float64(200),
				"message":                      "Profiles without project",
				"result.total":                 float64(10),
				"result.profiles.0.email":      test.ConstUserEmail,
				"result.profiles.0.alias":      "user",
				"result.profiles.0.profile_id": test.ConstUserID,
			},
			RequestHeaders: adminHeader,
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
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Members",
				"result.total": float64(11),
				//----------------------------------------
				"result.members.0.owner_id":      nil,
				"result.members.0.project_id":    nil,
				"result.members.0.active":        true,
				"result.members.0.member_id":     "4fc69519-b683-46f0-860c-3e7f12a17563",
				"result.members.0.online":        nil,
				"result.members.0.project_name":  "project3",
				"result.members.0.role":          float64(1),
				"result.members.0.schemes_count": float64(16),
				"result.members.0.name":          "Penny",
				"result.members.0.locked_at":     nil,
				"result.members.0.archived_at":   nil,
				"result.members.0.updated_at":    nil,
				"result.members.0.created_at":    "*",
				//----------------------------------------
				"result.members.1.owner_id":      nil,
				"result.members.1.project_id":    nil,
				"result.members.1.active":        true,
				"result.members.1.member_id":     "49a10a09-0bb3-48af-99cb-181533692585",
				"result.members.1.online":        true,
				"result.members.1.project_name":  "project3",
				"result.members.1.role":          float64(1),
				"result.members.1.schemes_count": float64(1),
				"result.members.1.name":          "Harrison",
				"result.members.1.locked_at":     nil,
				"result.members.1.archived_at":   nil,
				"result.members.1.updated_at":    nil,
				"result.members.1.created_at":    "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: ignored profile_id
			Name:       "test2_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                        float64(200),
				"message":                     "Members",
				"result.total":                float64(11),
				"result.members.0.member_id":  "*",
				"result.members.1.member_id":  "*",
				"result.members.11.member_id": nil,
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Member not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: search profiles without project
			Name:       "test2_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID, "search"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: search profiles without project for User UUID
			Name:       "test2_07",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, "search") + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_projectMember(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
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
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstUserProjectMemberID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstUserProjectMemberID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                float64(200),
				"message":             "Member",
				"result.member_id":    test.ConstUserProjectMemberID,
				"result.owner_id":     test.ConstAdminID,
				"result.owner_name":   "Penny",
				"result.project_id":   test.ConstAdminProject1ID,
				"result.project_name": "project1",
				"result.role":         float64(1),
				"result.profile_id":   test.ConstUserID,
				"result.name":         "Carly",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID, test.ConstUserProjectMemberID),
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
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID, test.ConstFakeID),
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
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID, test.ConstFakeID) + "?owner_id=" + test.ConstUserID,
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
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID, "4fc69519-b683-46f0-860c-3e7f12a17563") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                float64(200),
				"message":             "Member",
				"result.active":       true,
				"result.owner_id":     "c180ad5c-0c65-4cee-8725-12931cb5abb3",
				"result.project_id":   "d958ee44-a960-420e-9bbf-c7a35084c4aa",
				"result.member_id":    "4fc69519-b683-46f0-860c-3e7f12a17563",
				"result.profile_id":   "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
				"result.owner_name":   "Carly",
				"result.project_name": "project3",
				"result.role":         float64(1),
				"result.name":         "Penny",
				"result.created_at":   "*",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID, "4fc69519-b683-46f0-860c-3e7f12a17563"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                float64(200),
				"message":             "Member",
				"result.member_id":    "4fc69519-b683-46f0-860c-3e7f12a17563",
				"result.project_name": "project3",
				"result.role":         float64(1),
				"result.name":         "Penny",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID, test.ConstUserProjectMemberID),
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
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID, test.ConstFakeID),
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
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID, test.ConstFakeID) + "?owner_id=" + test.ConstUserID,
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
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID, test.ConstUserProjectMemberID) + "?owner_id=" + test.ConstUserID,
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

func TestHandler_addProjectMember(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
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
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":              float64(400),
				"message":           "Bad Request",
				"result.profile_id": "value is required",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"profile_id": "test",
			},
			Body: test.BodyTable{
				"code":              float64(400),
				"message":           "Bad Request",
				"result.profile_id": "value must be a valid UUID",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"profile_id": "51c12bb6-2da6-491d-8003-b024f54a1491",
			},
			Body: test.BodyTable{
				"code":             float64(200),
				"message":          "Member added",
				"result.member_id": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"profile_id": "51c12bb6-2da6-491d-8003-b024f54a1491",
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
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"profile_id": test.ConstFakeID,
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
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID) + "?owner_id=c180ad5c-0c65-4cee-8725-12931cb5abb3",
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"profile_id": "51c12bb6-2da6-491d-8003-b024f54a1491",
			},
			Body: test.BodyTable{
				"code":             float64(200),
				"message":          "Member added",
				"result.member_id": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_07",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"profile_id": "51c12bb6-2da6-491d-8003-b024f54a1491",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":              float64(400),
				"message":           "Bad Request",
				"result.profile_id": "value is required",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"profile_id": "test",
			},
			Body: test.BodyTable{
				"code":              float64(400),
				"message":           "Bad Request",
				"result.profile_id": "value must be a valid UUID",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "tes2_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"profile_id": "51c12bb6-2da6-491d-8003-b024f54a1491",
			},
			Body: test.BodyTable{
				"code":             float64(200),
				"message":          "Member added",
				"result.member_id": "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, test.ConstFakeID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"profile_id": "51c12bb6-2da6-491d-8003-b024f54a1491",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"profile_id": test.ConstFakeID,
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
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID) + "?owner_id=c180ad5c-0c65-4cee-8725-12931cb5abb3",
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"profile_id": "51c12bb6-2da6-491d-8003-b024f54a1491",
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
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"profile_id": "51c12bb6-2da6-491d-8003-b024f54a1491",
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

func TestHandler_updateProjectMember(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
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
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstFakeID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: broken member UUID
			Name:       "test1_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstFakeID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: broken member UUID
			Name:       "test1_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstFakeID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"active": true,
			},
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: update a member active
			Name:       "test1_03",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstUserProjectMemberID),
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
		{ // ADMIN: update a member active
			Name:       "test1_04",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstUserProjectMemberID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"role": 1,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID, "4fc69519-b683-46f0-860c-3e7f12a17563") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"role": 1,
			},
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_06",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID, "4fc69519-b683-46f0-860c-3e7f12a17563") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"role": 2,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_07",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID, "4fc69519-b683-46f0-860c-3e7f12a17563") + "?owner_id=" + test.ConstUserID,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"role": 3,
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: broken member UUID
			Name:       "test2_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID, test.ConstFakeID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: broken member UUID
			Name:       "test2_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID, test.ConstFakeID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"active": true,
			},
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: update a member active
			Name:       "test2_03",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID, "4fc69519-b683-46f0-860c-3e7f12a17563"),
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
		{ // USER: update a member active
			Name:       "test2_04",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID, "4fc69519-b683-46f0-860c-3e7f12a17563"),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"role": 1,
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstUserProjectMemberID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"role": 1,
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
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstUserProjectMemberID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"role": 1,
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_07",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstUserProjectMemberID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"active": true,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_removeProjectMember(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
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
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstUserProjectMemberID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstUserProjectMemberID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member deleted",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:           "test1_02",
			Method:         http.MethodDelete,
			Path:           test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstFakeID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:           "test1_03",
			Method:         http.MethodDelete,
			Path:           test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstUserProjectMemberID) + "?owner_id=" + test.ConstFakeID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID, "4fc69519-b683-46f0-860c-3e7f12a17563") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member deleted",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersProject, test.ConstUserProject1ID, "49a10a09-0bb3-48af-99cb-181533692585"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Member deleted",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:           "test2_02",
			Method:         http.MethodDelete,
			Path:           test.PathGluing(pathMembersProject, test.ConstUserProject1ID, test.ConstFakeID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:           "test2_03",
			Method:         http.MethodDelete,
			Path:           test.PathGluing(pathMembersProject, test.ConstUserProject1ID, test.ConstUserProjectMemberID) + "?owner_id=" + test.ConstFakeID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:           "test2_04",
			Method:         http.MethodDelete,
			Path:           test.PathGluing(pathMembersProject, test.ConstAdminProject1ID, test.ConstUserProjectMemberID) + "?owner_id=" + test.ConstAdminID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_projectMembersInvite(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathMembersInviteProject,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstAdminProject1ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgInviteNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstAdminProject1ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                        float64(200),
				"message":                     "Member invites",
				"result.total":                float64(13),
				"result.invites.0.status":     float64(2),
				"result.invites.0.email":      test.ConstUserEmail,
				"result.invites.0.name":       "user",
				"result.invites.0.surname":    "test1",
				"result.invites.0.updated_at": nil,
				"result.invites.0.created_at": "*",
				// --
				"result.invites.13.id": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgInviteNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstUserProject1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Member invites",
				"result.total": float64(3),
				// --
				"result.invites.0.status":     float64(2),
				"result.invites.0.email":      "admin@werbot.net",
				"result.invites.0.name":       "admin",
				"result.invites.0.surname":    "admin1",
				"result.invites.0.updated_at": nil,
				"result.invites.0.created_at": "*",
				// --
				"result.invites.1.status":     float64(1),
				"result.invites.1.email":      "user@werbot.net",
				"result.invites.1.name":       "user",
				"result.invites.1.surname":    "test1",
				"result.invites.1.updated_at": nil,
				"result.invites.1.created_at": "*",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER:
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgInviteNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstUserProject1ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Member invites",
				"result.total": float64(3),
				// --
				"result.invites.0.status":     float64(2),
				"result.invites.0.email":      "admin@werbot.net",
				"result.invites.0.name":       "admin",
				"result.invites.0.surname":    "admin1",
				"result.invites.0.updated_at": nil,
				"result.invites.0.created_at": "*",
				// --
				"result.invites.1.status":     float64(1),
				"result.invites.1.email":      "user@werbot.net",
				"result.invites.1.name":       "user",
				"result.invites.1.surname":    "test1",
				"result.invites.1.updated_at": nil,
				"result.invites.1.created_at": "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: ignored Owner UUID
			Name:       "test2_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstUserProject1ID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Member invites",
				"result.total": float64(3),
				// --
				"result.invites.0.status":     float64(2),
				"result.invites.0.email":      "admin@werbot.net",
				"result.invites.0.name":       "admin",
				"result.invites.0.surname":    "admin1",
				"result.invites.0.updated_at": nil,
				"result.invites.0.created_at": "*",
				// --
				"result.invites.1.status":     float64(1),
				"result.invites.1.email":      "user@werbot.net",
				"result.invites.1.name":       "user",
				"result.invites.1.surname":    "test1",
				"result.invites.1.updated_at": nil,
				"result.invites.1.created_at": "*",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_addProjectMemberInvite(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPost,
			Path:       pathMembersInviteProject,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstAdminProject1ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstAdminProject1ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.email":   "value is required",
				"result.name":    "value is required",
				"result.surname": "value is required",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstAdminProject1ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"email":   "test",
				"name":    "ab",
				"surname": "ab",
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.email":   "must be a valid email",
				"result.name":    "required field (3 to 30 characters)",
				"result.surname": "required field (3 to 30 characters)",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstAdminProject1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"email":   "user90@werbot.net",
				"name":    "User",
				"surname": "Name",
			},
			Body: test.BodyTable{
				"code":          float64(200),
				"message":       "Member invited",
				"result.token":  "*",
				"result.status": float64(1),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstUserProject1ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"email":   "user99@werbot.net",
				"name":    "User",
				"surname": "Name",
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
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstUserProject1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"email":   "user99@werbot.net",
				"name":    "User",
				"surname": "Name",
			},
			Body: test.BodyTable{
				"code":          float64(200),
				"message":       "Member invited",
				"result.token":  "*",
				"result.status": float64(1),
			},
			RequestHeaders: adminHeader,
		},

		{ // USER:
			Name:       "test2_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstUserProject1ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.email":   "value is required",
				"result.name":    "value is required",
				"result.surname": "value is required",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstUserProject1ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"email":   "test",
				"name":    "ab",
				"surname": "ab",
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.email":   "must be a valid email",
				"result.name":    "required field (3 to 30 characters)",
				"result.surname": "required field (3 to 30 characters)",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstUserProject1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"email":   "user99@werbot.net",
				"name":    "User",
				"surname": "Name",
			},
			Body: test.BodyTable{
				"code":          float64(200),
				"message":       "Member invited",
				"result.token":  "*",
				"result.status": float64(1),
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"email":   "user99@werbot.net",
				"name":    "User",
				"surname": "Name",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"email":   "user99@werbot.net",
				"name":    "User",
				"surname": "Name",
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

func TestHandler_deleteProjectMemberInvite(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodDelete,
			Path:       pathMembersInviteProject,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstAdminProject1ID, test.ConstAdminProject1InviteID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstAdminProject1ID, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstFakeID, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstAdminProject1ID, test.ConstAdminProject1InviteID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Invite deleted",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstUserProject1ID, test.ConstUserProject1InviteID) + "?owner_id" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstUserProject1ID, "15348c8a-894e-49e6-88a9-79aa058892f3") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Invite deleted",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER:
			Name:       "test2_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstUserProject1ID, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstFakeID, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_03",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstUserProject1ID, "3ec414fc-0bbe-44b8-b1d2-ff99171b4963"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Invite deleted",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstAdminProject1ID, test.ConstAdminProject1InviteID) + "?owner_id" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_05",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathMembersInviteProject, test.ConstAdminProject1ID, "15348c8a-894e-49e6-88a9-79aa058892f3") + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

// TODO 1. registration if there is no profile, 2. authorization if the profile is not authorized,
func TestHandler_membersInviteActivate(t *testing.T) {
	app, teardownTestCase, _, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // fake invite
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersInvite, test.ConstFakeID),
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // USER: request
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathMembersInvite, test.ConstUserMemberInviteID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Invite confirmed",
			},
			RequestHeaders: userHeader,
		},
		// TODO add other test cases to activate member invite
	}

	test.RunCaseAPITests(t, app, testTable)
}
