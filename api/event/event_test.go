package event

import (
	"net/http"
	"strings"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/pkg/crypto"
)

func TestHandler_events(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)
	adminHeader, userHeader := env.adminHeader, env.userHeader

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
			Path:           test.PathGluing(pathEvent, "abc", env.admin.ID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},

		{ // ADMIN: error displaying list of events with a broken profile UUID
			Name:           "test1_04",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEvent, "abc", env.admin.ID) + "?owner_id=" + crypto.NewPassword(8, false),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},

		// ---------------------------
		// profile event
		{ // ADMIN: error displaying list of events due to invalid arguments, ignoring broken limit
			Name:       "test1_11",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProfile, env.admin.ID) + "?limit=abc",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"result.total": float64(len(env.adminProfileEvents)),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: non-existent profile UUID
			Name:       "test1_12",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProfile, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Profile not found",
			},
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
			Path:       test.PathGluing(pathEventProfile, env.admin.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"result.total": float64(len(env.adminProfileEvents)),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: list of all profile events with limit
			Name:       "test1_15",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProfile, env.admin.ID) + "?limit=1",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"result.total":       float64(len(env.adminProfileEvents)),
				"result.events.1.id": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_16",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProfile, env.user.ID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Profile not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_17",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProfile, env.user.ID) + "?owner_id=" + env.user.ID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                       float64(200),
				"result.total":               float64(len(env.userProfileEvents)),
				"result.events.0.owner_id":   env.user.ID,
				"result.events.0.profile_id": env.user.ID,
				"result.events.0.section":    float64(1),
				"result.events.0.session.id": env.userProfileSess.ID,
				"result.events.0.session.ip": env.userProfileSess.IP,
				"result.events.0.type":       float64(9),
				// --
				"result.events.1.id": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // USER: error displaying profile events not owned by the profile
			Name:           "test2_11",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProfile, env.admin.ID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},

		// ---------------------------
		// project event
		{ // ADMIN: non-existent project UUID
			Name:           "test1_21",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProject, test.ConstFakeID),
			StatusCode:     404,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: list of all project events
			Name:       "test1_22",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProject, env.project.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                       float64(200),
				"result.total":               float64(len(env.projectEvents)),
				"result.events.0.project_id": env.project.ID,
				"result.events.1.project_id": env.project.ID,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: list of all project events with limit
			Name:       "test1_23",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProject, env.project.ID) + "?limit=1",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                       float64(200),
				"result.total":               float64(len(env.projectEvents)),
				"result.events.0.project_id": env.project.ID,
				"result.events.1.project_id": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: project events filtered by section
			Name:       "test1_24",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProject, env.project.ID) + "?section=1",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"result.total": float64(len(env.projectEvents)),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: project events filtered by type
			Name:       "test1_25",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProject, env.project.ID) + "?type=1",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                       float64(200),
				"result.total":               float64(1),
				"result.events.0.project_id": env.project.ID,
				"result.events.0.type":       float64(env.projectAndroid.Type),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: project events searched by user agent
			Name:       "test1_26",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProject, env.project.ID) + "?search=android",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                       float64(200),
				"result.total":               float64(1),
				"result.events.0.session.ip": env.projectAndroidS.IP,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: project events searched by IP
			Name:       "test1_27",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProject, env.project.ID) + "?search=2001:0db8",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                       float64(200),
				"result.total":               float64(1),
				"result.events.0.session.ip": env.projectIPv6S.IP,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: project events search with no matches
			Name:           "test1_28",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProject, env.project.ID) + "?search=no-such-value",
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: invalid event type is rejected
			Name:           "test1_29",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProject, env.project.ID) + "?type=99",
			StatusCode:     400,
			Body:           test.BodyInvalidArgument,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: negative section is rejected
			Name:           "test1_30",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProject, env.project.ID) + "?section=-1",
			StatusCode:     400,
			Body:           test.BodyInvalidArgument,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: search longer than the 128 character maximum is rejected
			Name:           "test1_35",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProject, env.project.ID) + "?search=" + strings.Repeat("a", 129),
			StatusCode:     400,
			Body:           test.BodyInvalidArgument,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: project events filtered by a section with no matches
			Name:           "test1_34",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProject, env.project.ID) + "?section=2",
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // USER: error displaying project events not owned by the profile
			Name:           "test2_21",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProject, env.project.ID),
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
			Path:       test.PathGluing(pathEventScheme, env.scheme.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                               float64(200),
				"result.total":                       float64(len(env.schemeEvents)),
				"result.events.0.owner_id":           env.admin.ID,
				"result.events.0.scheme_id":          env.scheme.ID,
				"result.events.0.section":            float64(1),
				"result.events.0.session.id":         env.schemeFirstS.ID,
				"result.events.0.session.user_agent": env.schemeFirstS.UserAgent,
				"result.events.0.session.ip":         env.schemeFirstS.IP,
				// --
				"result.events.11.session.id": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: list of all scheme events with limit
			Name:       "test1_33",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventScheme, env.scheme.ID) + "?limit=1",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                       float64(200),
				"result.total":               float64(len(env.schemeEvents)),
				"result.events.0.session.id": env.schemeFirstS.ID,
				"result.events.1.session.id": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // USER: error displaying scheme events not owned by the profile
			Name:           "test2_31",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventScheme, env.scheme.ID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_event(t *testing.T) {
	app, env, teardownTestCase := setupTest(t)
	defer teardownTestCase(t)
	adminHeader, userHeader := env.adminHeader, env.userHeader

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
			Path:           test.PathGluing(pathEvent, "abc", env.admin.ID, env.adminProfileEvent.ID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},

		// profile event
		{ // ADMIN: profile event information by UUID
			Name:       "test1_11",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProfile, env.admin.ID, env.adminProfileEvent.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                      float64(200),
				"result.profile_id":         env.admin.ID,
				"result.section":            float64(1),
				"result.type":               float64(env.adminProfileEvent.Type),
				"result.session.id":         env.adminProfileSess.ID,
				"result.session.ip":         env.adminProfileSess.IP,
				"result.session.user_agent": env.adminProfileSess.UserAgent,
				"result.meta_data":          "e30=",
				"result.created_at":         "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // USER: error displaying profile event info not owned by the profile
			Name:           "test2_11",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProfile, env.admin.ID, env.adminProfileEvent.ID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},

		// project event
		{ // ADMIN: project event information by UUID
			Name:       "test1_21",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventProject, env.project.ID, env.projectAndroid.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                      float64(200),
				"result.project_id":         env.project.ID,
				"result.section":            float64(1),
				"result.type":               float64(env.projectAndroid.Type),
				"result.session.id":         env.projectAndroidS.ID,
				"result.session.ip":         env.projectAndroidS.IP,
				"result.session.user_agent": env.projectAndroidS.UserAgent,
				"result.meta_data":          "e30=",
				"result.created_at":         "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // USER: error displaying project event info not owned by the profile
			Name:           "test2_21",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventProject, env.project.ID, env.projectAndroid.ID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},

		// scheme event
		{ // ADMIN: scheme event information by UUID
			Name:       "test1_31",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathEventScheme, env.scheme.ID, env.schemeFirst.ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                      float64(200),
				"result.scheme_id":          env.scheme.ID,
				"result.section":            float64(1),
				"result.type":               float64(env.schemeFirst.Type),
				"result.session.id":         env.schemeFirstS.ID,
				"result.session.ip":         env.schemeFirstS.IP,
				"result.session.user_agent": env.schemeFirstS.UserAgent,
				"result.meta_data":          "e30=",
				"result.created_at":         "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // USER: error displaying scheme event info not owned by the profile
			Name:           "test2_31",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathEventScheme, env.scheme.ID, env.schemeFirst.ID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		// TODO add other test cases to show event information from other categories
	}

	test.RunCaseAPITests(t, app, testTable)
}
