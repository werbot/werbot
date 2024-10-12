package scheme

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/uuid"
)

func TestHandler_schemes(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: broken request without project UUID
			Name:           "test1_01",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with fake UUID
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, "server"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with servers
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Servers",
				"result.total": float64(20),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with databases
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "database"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Databases",
				"result.total": float64(27),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with desktops
			Name:       "test1_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "desktop"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Desktops",
				"result.total": float64(6),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with containers
			Name:       "test1_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "container"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Containers",
				"result.total": float64(6),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with clouds
			Name:       "test1_07",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "cloud"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Clouds",
				"result.total": float64(15),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with applications
			Name:       "test1_08",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "application"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Applications",
				"result.total": float64(3),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request
			Name:           "test1_09",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "test"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with pagination
			Name:       "test1_10",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server") + "?limit=2&offset=0",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":             float64(200),
				"message":          "Servers",
				"result.total":     float64(20),
				"result.servers.2": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with fake profile UUID
			Name:       "test1_11",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with the project does not belong profile UUID
			Name:       "test1_12",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server") + "?owner_id=" + test.ConstUserID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with the project does belong profile UUID
			Name:       "test1_13",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, "server") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Servers",
				"result.total": float64(11),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with the project does belong profile UUID with pagination
			Name:       "test1_14",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, "server") + "?limit=2&offset=0&owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Servers",
				"result.total": float64(11),
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: broken request without project UUID
			Name:           "test2_01",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with fake UUID
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, "server"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: default request
			Name:       "test2_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, "server"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Servers",
				"result.total": float64(11),
			},
			RequestHeaders: userHeader,
		},
		{ // USER: default request with fake profile UUID
			Name:       "test2_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, "server") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Servers",
				"result.total": float64(11),
			},
			RequestHeaders: userHeader,
		},
		{ // USER: default request with the team does belong profile UUID
			Name:       "test2_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server") + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_scheme(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request with fake scheme UUID
			Name:           "test1_01",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstFakeID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with fake project UUID
			Name:           "test1_02",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstAdminSchemeSSH1ID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Scheme",
				"result.address":     "secret.net.google.com",
				"result.active":      nil,
				"result.online":      true,
				"result.audit":       nil,
				"result.project_id":  test.ConstAdminProject1ID,
				"result.auth_method": float64(1),
				"result.scheme_type": float64(103),
				"result.title":       "Git server",
				"result.alias":       "onxzU5",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with the server does not belong profile UUID
			Name:           "test1_04",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with the project does not belong profile UUID
			Name:           "test1_05",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with the project does belong profile UUID
			Name:       "test1_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Scheme",
				"result.active":      true,
				"result.address":     "127.0.0.4",
				"result.audit":       nil,
				"result.online":      nil,
				"result.project_id":  test.ConstUserProject1ID,
				"result.auth_method": float64(1),
				"result.scheme_type": float64(103),
				"result.title":       "Storage server backup",
				"result.alias":       "JiepgT",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request with fake scheme UUID
			Name:           "test2_01",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstFakeID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with fake project UUID
			Name:           "test2_02",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstUserSchemeSSH1ID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: base request
			Name:       "test2_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Scheme",
				"result.address":     "127.0.0.4",
				"result.active":      true,
				"result.audit":       nil,
				"result.online":      nil,
				"result.auth_method": float64(1),
				"result.scheme_type": float64(103),
				"result.title":       "Storage server backup",
				"result.alias":       "JiepgT",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: default request with the scheme does not belong profile UUID
			Name:           "test2_04",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: default request with the project does not belong profile UUID
			Name:           "test2_05",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: default request with the project does belong profile UUID
			Name:           "test2_06",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID) + "?owner_id=" + test.ConstAdminID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_addScheme(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: base request without access information
			Name:       "test1_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server"),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title":  "title",
				"audit":  true,
				"active": true,
				"scheme": test.BodyTable{},
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result": map[string]any{
					"scheme.access": "exactly one field is required in oneof",
				},
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request with broken scheme
			Name:       "test1_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server"),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title":  "title",
				"audit":  true,
				"active": true,
				"scheme": test.BodyTable{
					"test": test.BodyTable{
						"alias": "alias",
					},
				},
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result": map[string]any{
					"scheme.access": "exactly one field is required in oneof",
				},
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request
			Name:       "test1_04",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server"),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title": "title",
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"address": "domain,com", // broken
						"key": test.BodyTable{
							"login":  "login",
							"key_id": test.ConstFakeID,
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result": map[string]any{
					"scheme.server_ssh.address": "value must be a valid hostname, or ip address",
				},
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request
			Name:       "test1_05",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server"),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title": "title",
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"address": "127.0.0.300", // broken
						"key": test.BodyTable{
							"login":  "login",
							"key_id": test.ConstFakeID,
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result": map[string]any{
					"scheme.server_ssh.address": "value must be a valid hostname, or ip address",
				},
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request
			Name:       "test1_06",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server"),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title": "title",
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"address": "domain,com",    // broken
						"port":    float64(100000), // broken
						"key": test.BodyTable{
							"login":  "login",
							"key_id": test.ConstFakeID,
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result": map[string]any{
					"scheme.server_ssh.address": "value must be a valid hostname, or ip address",
					"scheme.server_ssh.port":    "value must be greater than or equal to 1 and less than 65536",
				},
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request
			Name:       "test1_07",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server"),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"title":  "title",
				"audit":  true,
				"active": true,
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias",
						"address": "127.0.0.1",
						"port":    float64(10000),
						"key": test.BodyTable{
							"login":  "login",
							"key_id": uuid.New(),
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Key not found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request with autogenerate (use key UUID - 00000000-0000-0000-0000-000000000000)
			Name:       "test1_08",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server"),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title":  "title",
				"audit":  true,
				"active": true,
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias",
						"address": "127.0.0.1",
						"port":    float64(10000),
						"key": test.BodyTable{
							"login":  "login",
							"key_id": test.ConstFakeID,
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":             float64(200),
				"message":          "Scheme added",
				"result.scheme_id": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request with generated key UUID
			Name:       "test1_09",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server"),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title":  "title",
				"audit":  true,
				"active": true,
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias",
						"address": "127.0.0.1",
						"port":    float64(10000),
						"key": test.BodyTable{
							"login":  "login",
							"key_id": "aac78aae-2036-468a-a12f-beadf7bd07ec",
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":             float64(200),
				"message":          "Scheme added",
				"result.scheme_id": "*",
			},
			RequestHeaders: adminHeader,
			PreWorkHook:    serverKeygen(app.Redis, "aac78aae-2036-468a-a12f-beadf7bd07ec", true),
		},
		{ // ADMIN: base request with password
			Name:       "test1_10",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server"),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title":  "title",
				"audit":  true,
				"active": true,
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias",
						"address": "127.0.0.1",
						"port":    float64(10000),
						"password": test.BodyTable{
							"login":    "login",
							"password": crypto.NewPassword(8, false),
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":             float64(200),
				"message":          "Scheme added",
				"result.scheme_id": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request with small password
			Name:       "test1_11",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server"),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title":  "title",
				"audit":  true,
				"active": true,
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias",
						"address": "127.0.0.1",
						"port":    float64(10000),
						"password": test.BodyTable{
							"login":    "login",
							"password": crypto.NewPassword(7, false), // broken
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result": map[string]any{
					"scheme.server_ssh.password.password": "value length must be at least 8 characters",
				},
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request with big password
			Name:       "test1_12",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server"),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title":  "title",
				"audit":  true,
				"active": true,
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias",
						"address": "127.0.0.1",
						"port":    float64(10000),
						"password": test.BodyTable{
							"login":    "login",
							"password": crypto.NewPassword(33, false), // broken
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result": map[string]any{
					"scheme.server_ssh.password.password": "value length must be at most 32 characters",
				},
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with fake project UUID
			Name:        "test1_13",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstFakeID, "server"),
			StatusCode:  404,
			RequestBody: test.BodyTable{},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with the project does not belong profile UUID
			Name:        "test1_14",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, "server"),
			StatusCode:  404,
			RequestBody: test.BodyTable{},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: default request with the project does belong profile UUID
			Name:       "test1_15",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, "server") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title":  "title",
				"audit":  true,
				"active": true,
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias",
						"address": "127.0.0.1",
						"port":    float64(10000),
						"key": test.BodyTable{
							"login":  "login",
							"key_id": test.ConstFakeID,
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":             float64(200),
				"message":          "Scheme added",
				"result.scheme_id": "*",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: base request
			Name:       "test2_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, "server"),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"title":  "title",
				"audit":  true,
				"active": true,
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias",
						"address": "127.0.0.1",
						"port":    float64(10000),
						"key": test.BodyTable{
							"login":  "login",
							"key_id": uuid.New(),
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Key not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: base request with autogenerate (use key UUID - 00000000-0000-0000-0000-000000000000)
			Name:       "test2_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, "server"),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title":  "title",
				"audit":  true,
				"active": true,
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias",
						"address": "127.0.0.1",
						"port":    float64(10000),
						"key": test.BodyTable{
							"login":  "login",
							"key_id": test.ConstFakeID,
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":             float64(200),
				"message":          "Scheme added",
				"result.scheme_id": "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: base request with generated key UUID
			Name:       "test2_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, "server"),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title":  "title",
				"audit":  true,
				"active": true,
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias",
						"address": "127.0.0.1",
						"port":    float64(10000),
						"key": test.BodyTable{
							"login":  "login",
							"key_id": "aac78aae-2036-468a-a12f-beadf7bd07ec",
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":             float64(200),
				"message":          "Scheme added",
				"result.scheme_id": "*",
			},
			RequestHeaders: userHeader,
			PreWorkHook:    serverKeygen(app.Redis, "aac78aae-2036-468a-a12f-beadf7bd07ec", true),
		},
		{ // USER: base request with password
			Name:       "test2_04",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, "server"),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title":  "title",
				"audit":  true,
				"active": true,
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias",
						"address": "127.0.0.1",
						"port":    float64(10000),
						"password": test.BodyTable{
							"login":    "login",
							"password": crypto.NewPassword(8, false),
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":             float64(200),
				"message":          "Scheme added",
				"result.scheme_id": "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with fake project UUID
			Name:        "test2_05",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstFakeID, "server"),
			StatusCode:  404,
			RequestBody: test.BodyTable{},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: default request with the project does not belong profile UUID
			Name:        "test2_06",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server"),
			StatusCode:  404,
			RequestBody: test.BodyTable{},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: default request with the project does belong profile UUID
			Name:       "test2_07",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, "server") + "?owner_id=" + test.ConstAdminID,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title":  "title",
				"audit":  true,
				"active": true,
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias",
						"address": "127.0.0.1",
						"port":    float64(10000),
						"key": test.BodyTable{
							"login":  "login",
							"key_id": test.ConstFakeID,
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_updateScheme(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request with broken scheme UUID
			Name:       "test1_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstFakeID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"audit": true,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID
			Name:       "test1_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstAdminSchemeSSH1ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"audit": true,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with the profile does not have access to the project UUID and scheme UUID
			Name:       "test1_03",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"audit": true,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with the profile does not have access to the scheme UUID
			Name:       "test1_04",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"audit": true,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with the profile has access to the project UUID and the scheme UUID
			Name:       "test1_05",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"audit": true,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request with active parameter
			Name:       "test1_06",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"active": false,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request with online parameter
			Name:       "test1_07",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"online": true,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request with online parameter
			Name:       "test1_08",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title": "",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.title": "value length must be at least 5 characters",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request with online parameter
			Name:       "test1_09",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title": "new title",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_10",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request with fake setting
			Name:       "test1_11",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"test": test.BodyTable{},
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request with info parameters
			Name:       "test1_12",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias2",
						"address": "127.0.0.255",
						"port":    float64(23456),
						"password": test.BodyTable{
							"login":    "newLogin",
							"password": "newPassword",
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request with info parameters
			Name:       "test1_13",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH2ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias2",
						"address": "127.0.0.255",
						"port":    float64(23456),
						"key": test.BodyTable{
							"login":  "newLogin",
							"key_id": test.ConstFakeID,
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request with info parameters
			Name:       "test1_14",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH2ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"key": test.BodyTable{
							"login":  "newLogin",
							"key_id": test.ConstFakeID,
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme updated",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request with broken scheme UUID
			Name:       "test2_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstFakeID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"audit": true,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID
			Name:       "test2_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstUserSchemeSSH1ID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"audit": true,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with the profile UUID (ignored) does not have access to the project UUID and scheme UUID
			Name:       "test2_03",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"audit": true,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme updated",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with the profile UUID does not have access UUID to the stranger scheme UUID
			Name:       "test2_04",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"audit": true,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with the profile UUID (ignored) has access to the strangers project UUID and the scheme UUID
			Name:       "test2_05",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"audit": true,
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: base request with active parameter
			Name:       "test2_06",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"active": true,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme updated",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: base request with online parameter
			Name:       "test2_07",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"online": true,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme updated",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: base request with online parameter
			Name:       "test2_08",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title": "",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.title": "value length must be at least 5 characters",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: base request with online parameter
			Name:       "test2_09",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title": "new title",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme updated",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request without parameters
			Name:       "test2_10",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: base request with fake setting
			Name:       "test2_11",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"test": test.BodyTable{},
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: base request with info parameters
			Name:       "test2_12",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias2",
						"address": "127.0.0.255",
						"port":    float64(23456),
						"password": test.BodyTable{
							"login":    "newLogin",
							"password": "newPassword",
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme updated",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: base request with info parameters
			Name:       "test2_13",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH2ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"alias":   "alias2",
						"address": "127.0.0.255",
						"port":    float64(23456),
						"key": test.BodyTable{
							"login":  "newLogin",
							"key_id": test.ConstFakeID,
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme updated",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: base request with info parameters
			Name:       "test2_14",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH2ID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"scheme": test.BodyTable{
					"server_ssh": test.BodyTable{
						"key": test.BodyTable{
							"login":  "newLogin",
							"key_id": test.ConstFakeID,
						},
					},
				},
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme updated",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_deleteScheme(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},

		{ // ADMIN: request with broken scheme UUID
			Name:       "test1_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID
			Name:       "test1_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstAdminSchemeSSH1ID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with the profile does not have access to the project UUID and scheme UUID
			Name:       "test1_03",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with the profile does not have access to the scheme UUID
			Name:       "test1_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with the profile has access to the project UUID and the scheme UUID
			Name:       "test1_05",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme deleted",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: base request with info parameters
			Name:       "test1_06",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme deleted",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request with broken scheme UUID
			Name:       "test2_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID
			Name:       "test2_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstUserSchemeSSH1ID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with the profile UUID (ignored) does not have access to the project UUID and scheme UUID
			Name:       "test2_03",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with the profile UUID (ignored) does not have access to the scheme UUID
			Name:       "test2_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgSchemeNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with the profile UUID (ignored) has access to the project UUID and the scheme UUID
			Name:       "test2_05",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH2ID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme deleted",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_schemeAccess(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "access"),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request with broken project UUID
			Name:           "test1_01",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstAdminSchemeSSH1ID, "access"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken scheme UUID
			Name:           "test1_02",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstFakeID, "access"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID and scheme UUID
			Name:           "test1_03",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "access"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID, scheme UUID and profile UUID
			Name:           "test1_04",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "access") + "?owner_id=" + test.ConstFakeID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID and scheme UUID
			Name:           "test1_05",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "access") + "?owner_id=" + test.ConstUserID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:           "test1_06",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "access") + "?owner_id=" + test.ConstUserID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:           "test1_07",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID, "access") + "?owner_id=" + test.ConstUserID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:           "test1_08",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID, "access") + "?owner_id=" + test.ConstUserID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:       "test1_09",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "access"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                                       float64(200),
				"message":                                    "Scheme access",
				"result.scheme.server_ssh.address":           "secret.net.google.com",
				"result.scheme.server_ssh.port":              float64(2206),
				"result.scheme.server_ssh.alias":             "onxzU5",
				"result.scheme.server_ssh.password.login":    "ubuntu6",
				"result.scheme.server_ssh.password.password": "***",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:       "test1_10",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "access") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                                       float64(200),
				"message":                                    "Scheme access",
				"result.scheme.server_ssh.address":           "127.0.0.4",
				"result.scheme.server_ssh.port":              float64(2207),
				"result.scheme.server_ssh.alias":             "JiepgT",
				"result.scheme.server_ssh.password.login":    "test",
				"result.scheme.server_ssh.password.password": "***",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request with broken project UUID
			Name:           "test2_01",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstUserSchemeSSH1ID, "access"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken scheme UUID
			Name:           "test2_02",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstFakeID, "access"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID and scheme UUID
			Name:           "test2_03",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "access"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID, scheme UUID and profile UUID
			Name:           "test2_04",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "access") + "?owner_id=" + test.ConstFakeID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID and scheme UUID
			Name:           "test2_05",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "access") + "?owner_id=" + test.ConstAdminID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with profile UUID (ignored)
			Name:       "test2_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "access") + "?owner_id=" + test.ConstAdminID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                                       float64(200),
				"message":                                    "Scheme access",
				"result.scheme.server_ssh.address":           "127.0.0.4",
				"result.scheme.server_ssh.port":              float64(2207),
				"result.scheme.server_ssh.alias":             "JiepgT",
				"result.scheme.server_ssh.password.login":    "test",
				"result.scheme.server_ssh.password.password": "***",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:           "test2_07",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID, "access") + "?owner_id=" + test.ConstAdminID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:           "test2_08",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID, "access") + "?owner_id=" + test.ConstAdminID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:       "test2_09",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "access"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                                       float64(200),
				"message":                                    "Scheme access",
				"result.scheme.server_ssh.address":           "127.0.0.4",
				"result.scheme.server_ssh.port":              float64(2207),
				"result.scheme.server_ssh.alias":             "JiepgT",
				"result.scheme.server_ssh.password.login":    "test",
				"result.scheme.server_ssh.password.password": "***",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_schemeActivity(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity"),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request with broken project UUID
			Name:           "test1_01",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstAdminSchemeSSH1ID, "activity"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken scheme UUID
			Name:           "test1_02",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstFakeID, "activity"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID and scheme UUID
			Name:           "test1_03",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "activity"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID, scheme UUID and profile UUID
			Name:           "test1_04",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "activity") + "?owner_id=" + test.ConstFakeID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID and scheme UUID
			Name:           "test1_05",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "activity") + "?owner_id=" + test.ConstUserID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:           "test1_06",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstUserID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:           "test1_07",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstUserID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:           "test1_08",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstUserID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:       "test1_09",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Scheme activity",
				"result.week.mon.0":  float64(0),
				"result.week.mon.12": float64(0),
				"result.week.tue.1":  float64(0),
				"result.week.tue.11": float64(0),
				"result.week.tue.13": float64(0),
				"result.week.tue.23": float64(0),
				"result.week.wed.2":  float64(0),
				"result.week.wed.10": float64(0),
				"result.week.wed.14": float64(0),
				"result.week.wed.22": float64(0),
				"result.week.thu.3":  float64(0),
				"result.week.thu.9":  float64(0),
				"result.week.thu.15": float64(0),
				"result.week.thu.21": float64(0),
				"result.week.fri.4":  float64(0),
				"result.week.fri.8":  float64(0),
				"result.week.fri.16": float64(0),
				"result.week.fri.20": float64(0),
				"result.week.sat.5":  float64(0),
				"result.week.sat.7":  float64(0),
				"result.week.sat.17": float64(0),
				"result.week.sat.19": float64(0),
				"result.week.sun.6":  float64(0),
				"result.week.sun.18": float64(0),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: activity for now
			Name:       "test1_10",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity", "now"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":        float64(200),
				"message":     "Scheme activity",
				"result.hour": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: activity for select hour (timestamp)
			Name:       "test1_11",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity", "1725885312000"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":        float64(200),
				"message":     "Scheme activity",
				"result.hour": true,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: activity for owner UUID
			Name:       "test1_12",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Scheme activity",
				"result.week.mon.6":  float64(0),
				"result.week.mon.18": float64(0),
				"result.week.tue.5":  float64(0),
				"result.week.tue.7":  float64(0),
				"result.week.tue.17": float64(0),
				"result.week.tue.19": float64(0),
				"result.week.wed.4":  float64(0),
				"result.week.wed.8":  float64(0),
				"result.week.wed.16": float64(0),
				"result.week.wed.20": float64(0),
				"result.week.thu.3":  float64(0),
				"result.week.thu.9":  float64(0),
				"result.week.thu.15": float64(0),
				"result.week.thu.21": float64(0),
				"result.week.fri.2":  float64(0),
				"result.week.fri.10": float64(0),
				"result.week.fri.14": float64(0),
				"result.week.fri.22": float64(0),
				"result.week.sat.1":  float64(0),
				"result.week.sat.11": float64(0),
				"result.week.sat.13": float64(0),
				"result.week.sat.23": float64(0),
				"result.week.sun.0":  float64(0),
				"result.week.sun.12": float64(0),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: activity for now for owner UUID
			Name:       "test1_13",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "activity", "now") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":        float64(200),
				"message":     "Scheme activity",
				"result.hour": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: activity for owner UUID select hour (timestamp)
			Name:       "test1_14",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "activity", "1725863712") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":        float64(200),
				"message":     "Scheme activity",
				"result.hour": false,
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request with broken project UUID
			Name:           "test2_01",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstUserSchemeSSH1ID, "activity"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken scheme UUID
			Name:           "test2_02",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstFakeID, "activity"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID and scheme UUID
			Name:           "test2_03",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "activity"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID, scheme UUID and profile UUID
			Name:           "test2_04",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "activity") + "?owner_id=" + test.ConstFakeID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID and scheme UUID
			Name:           "test2_05",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "activity") + "?owner_id=" + test.ConstAdminID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:       "test2_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstAdminID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Scheme activity",
				"result.week.mon.6":  float64(0),
				"result.week.mon.18": float64(0),
				"result.week.tue.5":  float64(0),
				"result.week.tue.7":  float64(0),
				"result.week.tue.17": float64(0),
				"result.week.tue.19": float64(0),
				"result.week.wed.4":  float64(0),
				"result.week.wed.8":  float64(0),
				"result.week.wed.16": float64(0),
				"result.week.wed.20": float64(0),
				"result.week.thu.3":  float64(0),
				"result.week.thu.9":  float64(0),
				"result.week.thu.15": float64(0),
				"result.week.thu.21": float64(0),
				"result.week.fri.2":  float64(0),
				"result.week.fri.10": float64(0),
				"result.week.fri.14": float64(0),
				"result.week.fri.22": float64(0),
				"result.week.sat.1":  float64(0),
				"result.week.sat.11": float64(0),
				"result.week.sat.13": float64(0),
				"result.week.sat.23": float64(0),
				"result.week.sun.0":  float64(0),
				"result.week.sun.12": float64(0),
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:           "test2_07",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstAdminID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:           "test2_08",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstAdminID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:           "test2_09",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: activity for now
			Name:       "test2_10",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "activity", "now"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":        float64(200),
				"message":     "Scheme activity",
				"result.hour": "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: activity for select hour (timestamp)
			Name:       "test2_11",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "activity", "1725863712"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":        float64(200),
				"message":     "Scheme activity",
				"result.hour": false,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: activity for owner UUID
			Name:           "test2_12",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstAdminID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: activity for now for owner UUID
			Name:           "test2_13",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity", "now") + "?owner_id=" + test.ConstAdminID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: activity for owner UUID select hour (timestamp)
			Name:           "test2_14",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity", "1725863712") + "?owner_id=" + test.ConstAdminID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_updateSchemeActivity(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	createRepeatedValues := func(size int, value int32) []int32 {
		repeatedValues := make([]int32, size)
		for i := range repeatedValues {
			repeatedValues[i] = value
		}
		return repeatedValues
	}

	createActivityTable := func(values []int32, brokenDay string, brokenValues []int32) test.BodyTable {
		activity := test.BodyTable{
			"mon": values,
			"tue": values,
			"wed": values,
			"thu": values,
			"fri": values,
			"sat": values,
			"sun": values,
		}

		if brokenDay != "" && brokenValues != nil {
			activity[brokenDay] = brokenValues
		}

		return test.BodyTable{"activity": activity}
	}

	validDayValues := createRepeatedValues(23, 0)
	validDayValuesBody := createActivityTable(validDayValues, "", nil)

	brokenDayValues := createRepeatedValues(22, 1)
	brokenDayValuesBody := createActivityTable(validDayValues, "mon", brokenDayValues)

	brokenDayValues2 := createRepeatedValues(23, 2)
	brokenDayValues2Body2 := createActivityTable(validDayValues, "mon", brokenDayValues2)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity"),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request with broken project UUID
			Name:           "test1_01",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstAdminSchemeSSH1ID, "access"),
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken scheme UUID
			Name:           "test1_02",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstFakeID, "access"),
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID and scheme UUID
			Name:           "test1_03",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "activity"),
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: empty body request
			Name:       "test1_04",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity"),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":            float64(400),
				"message":         "Bad Request",
				"result.activity": "value is required",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: broken body request (not all hours are transferred to monday (0-23))
			Name:        "test1_05",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity"),
			StatusCode:  400,
			RequestBody: brokenDayValuesBody,
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result": map[string]any{
					"activity.mon": "value must contain at least 23 item(s)",
				},
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: broken body request (invalid values ​​passed to monday)
			Name:        "test1_06",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity"),
			StatusCode:  400,
			RequestBody: brokenDayValues2Body2,
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result": map[string]any{
					"activity.mon[0]": "value must be in list [0, 1]",
				},
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: valid request
			Name:        "test1_07",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity"),
			StatusCode:  200,
			RequestBody: validDayValuesBody,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme activity updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID, scheme UUID and profile UUID
			Name:           "test1_08",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "activity") + "?owner_id=" + test.ConstFakeID,
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID and scheme UUID
			Name:           "test1_09",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "activity") + "?owner_id=" + test.ConstUserID,
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken profile UUID
			Name:           "test1_10",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstFakeID,
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:           "test1_11",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstUserID,
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:           "test1_12",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstUserID,
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: valid request
			Name:        "test1_13",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstUserID,
			StatusCode:  200,
			RequestBody: validDayValuesBody,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme activity updated",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request with broken project UUID
			Name:           "test2_01",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstUserSchemeSSH1ID, "access"),
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken scheme UUID
			Name:           "test2_02",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstFakeID, "access"),
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID and scheme UUID
			Name:           "test2_03",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "activity"),
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: empty body request
			Name:       "test2_04",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity"),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":            float64(400),
				"message":         "Bad Request",
				"result.activity": "value is required",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: broken body request (not all hours are transferred to monday (0-23))
			Name:        "test2_05",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "activity"),
			StatusCode:  400,
			RequestBody: brokenDayValuesBody,
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result": map[string]any{
					"activity.mon": "value must contain at least 23 item(s)",
				},
			},
			RequestHeaders: userHeader,
		},
		{ // USER: broken body request (invalid values ​​passed to monday)
			Name:        "test2_06",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "activity"),
			StatusCode:  400,
			RequestBody: brokenDayValues2Body2,
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result": map[string]any{
					"activity.mon[0]": "value must be in list [0, 1]",
				},
			},
			RequestHeaders: userHeader,
		},
		{ // USER: valid request
			Name:        "test2_07",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "activity"),
			StatusCode:  200,
			RequestBody: validDayValuesBody,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme activity updated",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID, scheme UUID and profile UUID
			Name:           "test2_08",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "activity") + "?owner_id=" + test.ConstFakeID,
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID and scheme UUID
			Name:           "test2_09",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "activity") + "?owner_id=" + test.ConstAdminID,
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken profile UUID (ignored)
			Name:        "test2_10",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstFakeID,
			StatusCode:  200,
			RequestBody: validDayValuesBody,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Scheme activity updated",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:           "test2_11",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstAdminID,
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:           "test2_12",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstAdminID,
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: valid request
			Name:           "test2_13",
			Method:         http.MethodPatch,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "activity") + "?owner_id=" + test.ConstAdminID,
			StatusCode:     404,
			RequestBody:    validDayValuesBody,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_schemeFirewall(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request with broken project UUID
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken scheme UUID
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstFakeID, "firewall"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID and scheme UUID
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: empty body request
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                 float64(200),
				"message":              "Scheme firewall",
				"result.country.total": float64(3),
				"result.network.total": float64(2),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID, scheme UUID and profile UUID
			Name:       "test1_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken profile UUID
			Name:       "test1_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:       "test1_07",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstUserID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:       "test1_08",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstUserID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: valid request
			Name:       "test1_09",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                 float64(200),
				"message":              "Scheme firewall",
				"result.country.total": float64(4),
				"result.network.total": float64(3),
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request with broken project UUID
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstUserSchemeSSH1ID, "firewall"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken scheme UUID
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstFakeID, "firewall"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID and scheme UUID
			Name:       "test2_03",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: empty body request
			Name:       "test2_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                 float64(200),
				"message":              "Scheme firewall",
				"result.country.total": float64(4),
				"result.network.total": float64(3),
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID, scheme UUID and profile UUID
			Name:       "test2_05",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken profile UUID (ignored)
			Name:       "test2_06",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstFakeID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                 float64(200),
				"message":              "Scheme firewall",
				"result.country.total": float64(4),
				"result.network.total": float64(3),
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:       "test2_07",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:       "test2_08",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // USER: valid request
			Name:       "test2_09",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallNotFound,
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_addSchemeFirewall(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	genSchemeFirewallData := func(scheme, value string) test.BodyTable {
		data := test.BodyTable{
			"owner_id":   test.ConstFakeID,
			"project_id": test.ConstFakeID,
			"scheme_id":  test.ConstFakeID,
		}

		switch scheme {
		case "country":
			data["country_code"] = value
		case "network":
			data["network"] = value
		}
		return data
	}

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: empty request
			Name:       "test1_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result": map[string]any{
					"record": "exactly one field is required in oneof",
				},
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:        "test1_02",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode:  200,
			RequestBody: genSchemeFirewallData("country", "CA"),
			Body: test.BodyTable{
				"code":              float64(200),
				"message":           "Country added",
				"result.country_id": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with double
			Name:        "test1_03",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "CA"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:        "test1_04",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode:  200,
			RequestBody: genSchemeFirewallData("network", "192.168.20.20"),
			Body: test.BodyTable{
				"code":              float64(200),
				"message":           "Network added",
				"result.network_id": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with double
			Name:        "test1_05",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("network", "192.168.20.20"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID
			Name:        "test1_06",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "CA"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken scheme UUID
			Name:        "test1_07",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstFakeID, "firewall"),
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "CA"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID and scheme UUID
			Name:        "test1_08",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall"),
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "CA"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID, scheme UUID and profile UUID
			Name:        "test1_09",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall") + "?owner_id=" + test.ConstFakeID,
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "CA"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken profile UUID
			Name:        "test1_10",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstFakeID,
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "CA"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:        "test1_11",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstUserID,
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "CA"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:        "test1_12",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstUserID,
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "CA"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: valid request
			Name:        "test1_13",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstUserID,
			StatusCode:  200,
			RequestBody: genSchemeFirewallData("country", "CA"),
			Body: test.BodyTable{
				"code":              float64(200),
				"message":           "Country added",
				"result.country_id": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: valid request
			Name:        "test1_14",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstUserID,
			StatusCode:  200,
			RequestBody: genSchemeFirewallData("network", "192.168.20.20"),
			Body: test.BodyTable{
				"code":              float64(200),
				"message":           "Network added",
				"result.network_id": "*",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: empty request
			Name:       "test2_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall"),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result": map[string]any{
					"record": "exactly one field is required in oneof",
				},
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:        "test2_02",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall"),
			StatusCode:  200,
			RequestBody: genSchemeFirewallData("country", "ES"),
			Body: test.BodyTable{
				"code":              float64(200),
				"message":           "Country added",
				"result.country_id": "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with double
			Name:        "test2_03",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall"),
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "ES"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:        "test2_04",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall"),
			StatusCode:  200,
			RequestBody: genSchemeFirewallData("network", "192.168.20.30"),
			Body: test.BodyTable{
				"code":              float64(200),
				"message":           "Network added",
				"result.network_id": "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with double
			Name:        "test2_05",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall"),
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("network", "192.168.20.30"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID
			Name:        "test2_06",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstUserSchemeSSH1ID, "firewall"),
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "ES"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken scheme UUID
			Name:        "test2_07",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstFakeID, "firewall"),
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "ES"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID and scheme UUID
			Name:        "test2_08",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall"),
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "ES"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID, scheme UUID and profile UUID
			Name:        "test2_09",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall") + "?owner_id=" + test.ConstFakeID,
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "ES"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken profile UUID
			Name:        "test2_10",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstFakeID,
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "ES"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:        "test2_11",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstAdminID,
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "ES"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:        "test2_12",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstUserID,
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "ES"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: valid request
			Name:        "test2_13",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstAdminID,
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("country", "ES"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: valid request
			Name:        "test2_14",
			Method:      http.MethodPost,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstAdminID,
			StatusCode:  400,
			RequestBody: genSchemeFirewallData("network", "192.168.20.30"),
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  trace.MsgFailedToAdd,
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_updateSchemeFirewall(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	genRequestBody := func(scheme string, status bool) test.BodyTable {
		data := test.BodyTable{
			"owner_id":   test.ConstFakeID,
			"project_id": test.ConstFakeID,
			"scheme_id":  test.ConstFakeID,
		}

		switch scheme {
		case "country":
			data["country"] = status
		case "network":
			data["network"] = status
		}
		return data
	}

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},

		{ // ADMIN: request
			Name:       "test1_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":          float64(400),
				"message":       "Bad Request",
				"result.status": "exactly one field is required in oneof",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: update status list for country
			Name:        "test1_02",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode:  200,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Record updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: update status list for network
			Name:        "test1_03",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode:  200,
			RequestBody: genRequestBody("network", true),
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Record updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken scheme UUID
			Name:        "test1_04",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstFakeID, "firewall"),
			StatusCode:  404,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallListNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID
			Name:        "test1_05",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode:  404,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallListNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID, scheme UUID
			Name:        "test1_06",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall"),
			StatusCode:  404,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallListNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID, scheme UUID and profile UUID
			Name:        "test1_07",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall") + "?owner_id=" + test.ConstFakeID,
			StatusCode:  404,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallListNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken profile UUID
			Name:        "test1_08",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstFakeID,
			StatusCode:  404,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallListNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:        "test1_09",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstUserID,
			StatusCode:  404,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallListNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:        "test1_10",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstUserID,
			StatusCode:  404,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallListNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:        "test1_11",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstUserID,
			StatusCode:  200,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Record updated",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request
			Name:       "test2_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall"),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":          float64(400),
				"message":       "Bad Request",
				"result.status": "exactly one field is required in oneof",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: update status list for country
			Name:        "test2_02",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall"),
			StatusCode:  200,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Record updated",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: update status list for network
			Name:        "test2_03",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall"),
			StatusCode:  200,
			RequestBody: genRequestBody("network", true),
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Record updated",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken scheme UUID
			Name:        "test2_04",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstFakeID, "firewall"),
			StatusCode:  404,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallListNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID
			Name:        "test2_05",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstUserSchemeSSH1ID, "firewall"),
			StatusCode:  404,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallListNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID, scheme UUID
			Name:        "test2_06",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall"),
			StatusCode:  404,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallListNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID, scheme UUID and profile UUID
			Name:        "test2_07",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall") + "?owner_id=" + test.ConstFakeID,
			StatusCode:  404,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallListNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken profile UUID
			Name:        "test2_08",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstFakeID,
			StatusCode:  200,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Record updated",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:        "test2_09",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstAdminID,
			StatusCode:  404,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallListNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:        "test2_10",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstAdminID,
			StatusCode:  404,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallListNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:        "test2_11",
			Method:      http.MethodPatch,
			Path:        test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall") + "?owner_id=" + test.ConstAdminID,
			StatusCode:  404,
			RequestBody: genRequestBody("country", true),
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgFirewallListNotFound,
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_deleteSchemeFirewall(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall"),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request
			Name:           "test1_01",
			Method:         http.MethodDelete,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall", "country"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:           "test1_02",
			Method:         http.MethodDelete,
			Path:           test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall", "network"),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: remove country
			Name:       "test1_03",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall", "country", test.ConstAdminSchemeSSH1FrwCnt1ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Country deleted",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:       "test1_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall", "country", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: remove network
			Name:       "test1_05",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall", "network", test.ConstAdminSchemeSSH1FrwNtw1ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Network deleted",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:       "test1_06",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall", "network", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgNetworkNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID
			Name:       "test1_07",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstAdminSchemeSSH1ID, "firewall", "country", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken scheme UUID
			Name:       "test1_08",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstFakeID, "firewall", "country", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID and scheme UUID
			Name:       "test1_09",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall", "country", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken project UUID, scheme UUID and profile UUID
			Name:       "test1_10",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall", "country", test.ConstFakeID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken profile UUID
			Name:       "test1_11",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall", "country", test.ConstFakeID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:       "test1_12",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID, "firewall", "country", test.ConstFakeID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:       "test1_13",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstUserSchemeSSH1ID, "firewall", "country", test.ConstUserSchemeSSH1FrwCnt1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: valid request
			Name:       "test1_14",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall", "country", test.ConstUserSchemeSSH1FrwCnt1ID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Country deleted",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: remove country
			Name:       "test2_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall", "country", test.ConstUserSchemeSSH1FrwCnt2ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Country deleted",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:       "test2_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall", "country", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: remove network
			Name:       "test2_03",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall", "network", test.ConstUserSchemeSSH1FrwNtw2ID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Network deleted",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:       "test2_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall", "network", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgNetworkNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID
			Name:       "test2_05",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstUserSchemeSSH1ID, "firewall", "country", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken scheme UUID
			Name:       "test2_06",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstFakeID, "firewall", "country", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID and scheme UUID
			Name:       "test2_07",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall", "country", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken project UUID, scheme UUID and profile UUID
			Name:       "test2_08",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstFakeID, test.ConstFakeID, "firewall", "country", test.ConstFakeID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with broken profile UUID
			Name:       "test2_09",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstUserSchemeSSH1ID, "firewall", "country", test.ConstFakeID) + "?owner_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:       "test2_10",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID, "firewall", "country", test.ConstFakeID) + "?owner_id=" + test.ConstUserID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request
			Name:       "test2_11",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstUserProject1ID, test.ConstAdminSchemeSSH1ID, "firewall", "country", test.ConstAdminSchemeSSH1FrwCnt1ID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: userHeader,
		},
		{ // USER: valid request
			Name:       "test2_12",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathSchemes, test.ConstAdminProject1ID, test.ConstAdminSchemeSSH1ID, "firewall", "country", test.ConstAdminSchemeSSH1FrwCnt1ID) + "?owner_id=" + test.ConstAdminID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  trace.MsgCountryNotFound,
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}
