package token

/*
import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_projectMembersInvite(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathInvitesProject,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathInvitesProject, test.ConstAdminProject1ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathInvitesProject, test.ConstFakeID),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstAdminProject1ID),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstUserID,
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstUserProject1ID) + "?owner_id=" + test.ConstUserID,
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstFakeID),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstUserProject1ID),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstUserProject1ID) + "?owner_id=" + test.ConstAdminID,
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
			Path:       pathInvitesProject,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathInvitesProject, test.ConstAdminProject1ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathInvitesProject, test.ConstAdminProject1ID),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstAdminProject1ID),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstAdminProject1ID),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstUserProject1ID) + "?owner_id=" + test.ConstFakeID,
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstUserProject1ID) + "?owner_id=" + test.ConstUserID,
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstUserProject1ID),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstUserProject1ID),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstUserProject1ID),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstFakeID,
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstAdminProject1ID) + "?owner_id=" + test.ConstUserID,
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
			Path:       pathInvitesProject,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // unauthorized request
			Name:       "test0_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathInvitesProject, test.ConstAdminProject1ID, test.ConstAdminProject1InviteID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathInvitesProject, test.ConstAdminProject1ID, test.ConstFakeID),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstFakeID, test.ConstFakeID),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstAdminProject1ID, test.ConstAdminProject1InviteID),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstUserProject1ID, test.ConstUserProject1InviteID) + "?owner_id" + test.ConstFakeID,
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstUserProject1ID, "15348c8a-894e-49e6-88a9-79aa058892f3") + "?owner_id=" + test.ConstUserID,
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstUserProject1ID, test.ConstFakeID),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstFakeID, test.ConstFakeID),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstUserProject1ID, "3ec414fc-0bbe-44b8-b1d2-ff99171b4963"),
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstAdminProject1ID, test.ConstAdminProject1InviteID) + "?owner_id" + test.ConstFakeID,
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
			Path:       test.PathGluing(pathInvitesProject, test.ConstAdminProject1ID, "15348c8a-894e-49e6-88a9-79aa058892f3") + "?owner_id=" + test.ConstAdminID,
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
*/
