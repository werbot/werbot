package event

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/pkg/crypto"
)

func TestHandler_events(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathEvent,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // ADMIN: request without parameters
			Name:           "test1_01",
			Method:         http.MethodGet,
			Path:           pathEvent,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:           "test1_02",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEvent, "abc"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: error displaying list of events with a fake name
			Name:           "test1_03",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEvent, "abc", test.ConstAdminID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},

		{ // ADMIN: error displaying list of events with a broken profile UUID
			Name:           "test1_04",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEvent, "abc", test.ConstAdminID) + "?profile_id=" + crypto.NewPassword(8, false),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},

		// ---------------------------
		// profile event
		{ // ADMIN: error displaying list of events due to invalid arguments, ignoring broken limit
			Name:       "test1_11",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProfile, test.ConstAdminID) + "?limit=abc",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"result.total": float64(2),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: non-existent profile UUID
			Name:           "test1_12",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProfile, test.ConstFakeID),
			StatusCode:     404,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: broken profile UUID
			Name:           "test1_13",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProfile, crypto.NewPassword(8, false)),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: list of all profile events
			Name:       "test1_14",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProfile, test.ConstAdminID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"result.total": float64(2),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: list of all profile events with limit
			Name:       "test1_15",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProfile, test.ConstAdminID) + "?limit=1",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                float64(200),
				"result.total":        float64(2),
				"result.records.1.id": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // USER: error displaying profile events not owned by the profile
			Name:           "test2_11",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProfile, test.ConstAdminID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},

		// ---------------------------
		// project event
		{ // ADMIN: non-existent project UUID
			Name:           "test1_11",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProject, test.ConstFakeID),
			StatusCode:     404,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: list of all project events
			Name:       "test1_22",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProject, test.ConstAdminEventProjectID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                        float64(200),
				"result.total":                float64(2),
				"result.records.0.project_id": test.ConstAdminProjectEventID,
				"result.records.1.project_id": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: list of all project events with limit
			Name:       "test1_23",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProject, test.ConstAdminEventProjectID) + "?limit=1",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                        float64(200),
				"result.total":                float64(2),
				"result.records.0.project_id": test.ConstAdminProjectEventID,
				"result.records.1.project_id": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // USER: error displaying project events not owned by the profile
			Name:           "test2_21",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProject, test.ConstAdminEventProjectID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},

		// ---------------------------
		// scheme event
		{ // ADMIN: non-existent scheme UUID
			Name:           "test1_31",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventScheme, test.ConstFakeID),
			StatusCode:     404,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: list of all scheme events
			Name:       "test1_32",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventScheme, test.ConstAdminEventSchemeID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                         float64(200),
				"result.total":                 float64(11),
				"result.records.0.session_id":  "*",
				"result.records.11.session_id": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: list of all scheme events with limit
			Name:       "test1_33",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventScheme, test.ConstAdminEventSchemeID) + "?limit=1",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                        float64(200),
				"result.total":                float64(11),
				"result.records.0.session_id": "*",
				"result.records.1.session_id": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // USER: error displaying scheme events not owned by the profile
			Name:           "test2_31",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventScheme, test.ConstAdminEventSchemeID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		// TODO add other test cases to show events from other categories
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_event(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathEvent,
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // ADMIN: error displaying information about events with fake name
			Name:           "test1_01",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEvent, "abc", test.ConstAdminID, test.ConstAdminProfileEventID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},

		// profile event
		{ // ADMIN: profile event information by UUID
			Name:       "test1_11",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProfile, test.ConstAdminID, test.ConstAdminProfileEventID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":              float64(200),
				"result.user_id":    test.ConstAdminID,
				"result.profile_id": test.ConstAdminProfileEventID,
				"result.event":      float64(9),
				"result.ip":         "2001:db8:85a3::8a2e:370:7334",
				"result.meta_data":  "e30=",
				"result.section":    float64(1),
				"result.session_id": "98e3ddfc-dab0-4d4e-b48e-ab1717acae8b",
				"result.user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.9; rv:35.0) Gecko/20100101 Firefox/35.",
				"result.created_at": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // USER: error displaying profile event info not owned by the profile
			Name:           "test2_11",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProfile, test.ConstAdminID, test.ConstAdminProfileEventID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},

		// project event
		{ // ADMIN: project event information by UUID
			Name:       "test1_21",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProject, test.ConstAdminEventProjectID, test.ConstAdminProjectEventID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":              float64(200),
				"result.profile_id": test.ConstAdminID,
				"result.project_id": test.ConstAdminEventProjectID,
				"result.event":      float64(1),
				"result.ip":         "192.168.0.1",
				"result.meta_data":  "e30=",
				"result.section":    float64(1),
				"result.session_id": "98e3ddfc-dab0-4d4e-b48e-ab1717acae8b",
				"result.user_agent": "Mozilla/5.0 (Linux; U; Android 4.0.4; en-us; KFJWI Build/IMM76D) AppleWebKit/537.36 (KHTML, like Gecko) Silk/3.68 like Chrome/39.0.2171.93 Safari/537.36",
				"result.created_at": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // USER: error displaying project event info not owned by the profile
			Name:           "test2_21",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProject, test.ConstAdminEventProjectID, test.ConstAdminProjectEventID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},

		// scheme event
		{ // ADMIN: scheme event information by UUID
			Name:       "test1_31",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventScheme, test.ConstAdminEventSchemeID, test.ConstAdminSchemeEventID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":              float64(200),
				"result.profile_id": test.ConstAdminID,
				"result.scheme_id":  test.ConstAdminEventSchemeID,
				"result.event":      float64(1),
				"result.ip":         "192.168.1.1",
				"result.meta_data":  "e30=",
				"result.section":    float64(1),
				"result.session_id": "98e3ddfc-dab0-4d4e-b48e-ab1717acae8b",
				"result.user_agent": "Mozilla/5.0 (Windows NT 6.3; WOW64; Trident/7.0; Touch; LCJB; rv:11.0) like Gecko",
				"result.created_at": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // USER: error displaying scheme event info not owned by the profile
			Name:           "test2_31",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventScheme, test.ConstAdminEventSchemeID, test.ConstAdminSchemeEventID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		// TODO add other test cases to show event information from other categories
	}

	test.RunCaseAPITests(t, app, testTable)
}
