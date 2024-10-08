package key

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/utils/fsutil"
)

func TestHandler_generateNewKey(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathKeysGenerate,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: generate new key
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       pathKeysGenerate,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                float64(200),
				"message":             "New ssh key",
				"result.finger_print": "*",
				"result.key_type":     float64(1),
				"result.public":       "*",
				"result.uuid":         "*",
				"result.passphrase":   nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // USER: generate new key
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       pathKeysGenerate,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                float64(200),
				"message":             "New ssh key",
				"result.finger_print": "*",
				"result.key_type":     float64(1),
				"result.public":       "*",
				"result.uuid":         "*",
				"result.passphrase":   nil,
			},
			RequestHeaders: userHeader,
		},
		// TODO add other test cases to generate new key
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_keys(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathKeys,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       pathKeys,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Keys",
				"result.total": float64(12),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with pagination parameters
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       pathKeys + "?limit=2&offset=0",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Keys",
				"result.total": float64(12),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with incorrect user UUID parameter
			Name:           "test1_03",
			Method:         http.MethodGet,
			Path:           pathKeys + "?user_id=" + test.ConstFakeID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with broken user UUID parameter
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       pathKeys + "?user_id=" + crypto.NewPassword(8, false),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Keys",
				"result.total": float64(12),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with pagination and valid user UUID parameters
			Name:       "test1_05",
			Method:     http.MethodGet,
			Path:       pathKeys + "?limit=2&offset=0&user_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                             float64(200),
				"message":                          "Keys",
				"result.total":                     float64(2),
				"result.public_keys.0.fingerprint": "*",
				"result.public_keys.0.key":         "*",
				"result.public_keys.0.key_id":      "*",
				"result.public_keys.0.user_id":     "*",
				"result.public_keys.0.title":       "*",
				"result.public_keys.0.locked_at":   nil,
				"result.public_keys.0.archived_at": nil,
				"result.public_keys.0.updated_at":  nil,
				"result.public_keys.0.created_at":  "*",
				"result.public_keys.1.fingerprint": "*",
				"result.public_keys.1.key":         "*",
				"result.public_keys.1.key_id":      "*",
				"result.public_keys.1.user_id":     "*",
				"result.public_keys.1.title":       "*",
				"result.public_keys.1.locked_at":   "*",
				"result.public_keys.1.archived_at": nil,
				"result.public_keys.1.updated_at":  "*",
				"result.public_keys.1.created_at":  "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with valid user UUID parameter
			Name:       "test1_06",
			Method:     http.MethodGet,
			Path:       pathKeys + "?user_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Keys",
				"result.total": float64(2),
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request without parameters
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       pathKeys + "?limit=1&offset=0",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                             float64(200),
				"message":                          "Keys",
				"result.total":                     float64(1),
				"result.public_keys.0.fingerprint": "*",
				"result.public_keys.0.key":         nil,
				"result.public_keys.0.key_id":      "*",
				"result.public_keys.0.user_id":     nil,
				"result.public_keys.0.title":       "*",
				"result.public_keys.0.locked_at":   nil,
				"result.public_keys.0.archived_at": nil,
				"result.public_keys.0.updated_at":  nil,
				"result.public_keys.0.created_at":  "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with valid user UUID parameter
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       pathKeys + "?user_id=" + test.ConstAdminID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                             float64(200),
				"message":                          "Keys",
				"result.total":                     float64(1),
				"result.public_keys.0.fingerprint": "*",
				"result.public_keys.0.key":         nil,
				"result.public_keys.0.key_id":      "*",
				"result.public_keys.0.user_id":     nil,
				"result.public_keys.0.title":       "*",
				"result.public_keys.0.locked_at":   nil,
				"result.public_keys.0.archived_at": nil,
				"result.public_keys.0.updated_at":  nil,
				"result.public_keys.0.created_at":  "*",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_key(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathKeys, test.ConstAdminKeyID),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request with broken key UUID parameter
			Name:           "test1_01",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathKeys, crypto.NewPassword(8, false)),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with key UUID parameter
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathKeys, test.ConstAdminKeyID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Key",
				"result.fingerprint": "*",
				"result.key":         "*",
				"result.key_id":      "*",
				"result.user_id":     test.ConstAdminID,
				"result.title":       "public_key 1",
				"result.locked_at":   nil,
				"result.archived_at": nil,
				"result.updated_at":  nil,
				"result.created_at":  "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with key UUID and user UUID parameter
			Name:           "test1_03",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathKeys, test.ConstAdminKeyID) + "?user_id=" + test.ConstUserID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with key UUID and user UUID parameter
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathKeys, test.ConstUserKeyID1) + "?user_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Key",
				"result.title":       "public_key 4",
				"result.fingerprint": "*",
				"result.key":         "*",
				"result.key_id":      "*",
				"result.user_id":     test.ConstUserID,
				"result.locked_at":   "*",
				"result.archived_at": nil,
				"result.updated_at":  "*",
				"result.created_at":  "*",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request with key UUID parameter
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathKeys, test.ConstUserKeyID2),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Key",
				"result.fingerprint": "*",
				"result.key":         nil,
				"result.key_id":      "*",
				"result.user_id":     nil,
				"result.title":       "public_key 5",
				"result.locked_at":   nil,
				"result.archived_at": nil,
				"result.updated_at":  nil,
				"result.created_at":  "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with key UUID and user UUID parameter
			Name:           "test2_02",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathKeys, test.ConstAdminKeyID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request key UUID and user UUID parameter
			Name:           "test2_03",
			Method:         http.MethodGet,
			Path:           test.PathGluing(pathKeys, test.ConstAdminKeyID) + "?user_id=" + test.ConstAdminID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_addKey(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPost,
			Path:       pathKeys,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:       "test1_01",
			Method:     http.MethodPost,
			Path:       pathKeys,
			StatusCode: 400,
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.key":   "value is required",
				"result.title": "value is required",
			},
			RequestHeaders: adminHeader,
		},

		{ // ADMIN: request without parameters
			Name:       "test1_02",
			Method:     http.MethodPost,
			Path:       pathKeys,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title": "ab",
				"key":   "abc",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.title": "value length must be at least 3 characters",
				"result.key":   "value length must be at least 70 characters",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request adding a key to ADMIN profile
			Name:       "test1_03",
			Method:     http.MethodPost,
			Path:       pathKeys,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title": "Test key",
				"key":   string(fsutil.MustReadFile("../../fixtures/keys/users/user1/id_ed25519.pub")),
			},
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Key added",
				"result.fingerprint": "6e:cc:de:6c:a7:65:84:9e:7f:32:e3:5a:0b:82:27:89",
				"result.key_id":      "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request a repeating key
			Name:       "test1_04",
			Method:     http.MethodPost,
			Path:       pathKeys,
			StatusCode: 409,
			RequestBody: test.BodyTable{
				"title": "Test key",
				"key":   string(fsutil.MustReadFile("../../fixtures/keys/users/user1/id_ed25519.pub")),
			},
			Body: test.BodyTable{
				"code":    float64(409),
				"message": "Conflict",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request adding a key to another user by user UUID
			Name:       "test1_05",
			Method:     http.MethodPost,
			Path:       pathKeys + "?user_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title": "Test key",
				"key":   string(fsutil.MustReadFile("../../fixtures/keys/users/user1/id_ed25519.pub")),
			},
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Key added",
				"result.fingerprint": "6e:cc:de:6c:a7:65:84:9e:7f:32:e3:5a:0b:82:27:89",
				"result.key_id":      "*",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request a repeating key (the key was added by the admin earlier)
			// the test depends on the test "ADMIN: Authorized request adding a key to another user by user UUID"
			Name:       "test2_01",
			Method:     http.MethodPost,
			Path:       pathKeys,
			StatusCode: 409,
			RequestBody: test.BodyTable{
				"title": "Test key",
				"key":   string(fsutil.MustReadFile("../../fixtures/keys/users/user1/id_ed25519.pub")),
			},
			Body: test.BodyTable{
				"code":    float64(409),
				"message": "Conflict",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request adding a key
			Name:       "test2_02",
			Method:     http.MethodPost,
			Path:       pathKeys,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title": "Test key2",
				"key":   string(fsutil.MustReadFile("../../fixtures/keys/users/user2/id_ed25519.pub")),
			},
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Key added",
				"result.fingerprint": "bd:e6:3a:03:9f:29:7a:9c:fa:d0:f5:63:aa:16:a6:46",
				"result.key_id":      "*",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with valid user UUID and new key
			// The key will be added to the USER profile.
			Name:       "test2_03",
			Method:     http.MethodPost,
			Path:       pathKeys + "?user_id=" + test.ConstAdminID,
			StatusCode: 409,
			RequestBody: test.BodyTable{
				"title": "Test key",
				"key":   string(fsutil.MustReadFile("../../fixtures/keys/users/user1/id_ed25519.pub")),
			},
			Body: test.BodyTable{
				"code":    float64(409),
				"message": "Conflict",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request with valid user UUID and double key
			// The key will be added to the USER profile.
			Name:       "test2_04",
			Method:     http.MethodPost,
			Path:       pathKeys,
			StatusCode: 409,
			RequestBody: test.BodyTable{
				"user_id": test.ConstAdminID,
				"title":   "Test key",
				"key":     string(fsutil.MustReadFile("../../fixtures/keys/users/user1/id_ed25519.pub")),
			},
			Body: test.BodyTable{
				"code":    float64(409),
				"message": "Conflict",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request adding a key to another user by user UUID (ignored)
			Name:       "test2_05",
			Method:     http.MethodPost,
			Path:       pathKeys + "?user_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title": "Test key2",
				"key":   string(fsutil.MustReadFile("../../fixtures/keys/users/user3/id_ed25519.pub")),
			},
			Body: test.BodyTable{
				"code":               float64(200),
				"message":            "Key added",
				"result.fingerprint": "a2:e8:70:60:58:28:85:ba:d8:b6:eb:88:f0:00:20:7a",
				"result.key_id":      "*",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_updateKey(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPatch,
			Path:       pathKeys,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:           "test1_01",
			Method:         http.MethodPatch,
			Path:           pathKeys,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameter
			Name:       "test1_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathKeys, test.ConstAdminKeyID),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.title": "value is required",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request without parameter
			Name:       "test1_03",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathKeys, test.ConstAdminKeyID),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"title": "ab",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.title": "value length must be at least 3 characters",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with parameters
			Name:       "test1_04",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathKeys, test.ConstAdminKeyID),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title": "test key name",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Key updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request with parameters to another user by user UUID
			Name:       "test1_05",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathKeys, test.ConstUserKeyID1) + "?user_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"title": "test key name",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Key updated",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request with parameters and outsider key UUID
			Name:       "test2_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathKeys, test.ConstAdminKeyID),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"title": "test key name",
			},
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request with parameters and outsider key UUID and user UUID
			Name:       "test2_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathKeys, test.ConstAdminKeyID) + "?user_id=" + test.ConstAdminID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"title": "test key name",
			},
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_deleteKey(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodDelete,
			Path:       pathKeys,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: request without parameters
			Name:           "test1_01",
			Method:         http.MethodDelete,
			Path:           pathKeys,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request
			Name:       "test1_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathKeys, test.ConstAdminKeyID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Key removed",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: authorized request other USER
			Name:           "test1_03",
			Method:         http.MethodDelete,
			Path:           test.PathGluing(pathKeys, test.ConstUserKeyID1),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: request other USER with user UUID
			Name:       "test1_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathKeys, test.ConstUserKeyID1) + "?user_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Key removed",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: request
			Name:       "test2_01",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathKeys, test.ConstUserKeyID1),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Key removed",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: request other USER
			Name:           "test2_02",
			Method:         http.MethodDelete,
			Path:           test.PathGluing(pathKeys, test.ConstAdminKeyID),
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
		{ // USER: request other USER with user UUID
			Name:           "test2_03",
			Method:         http.MethodDelete,
			Path:           test.PathGluing(pathKeys, test.ConstAdminKeyID) + "?user_id=" + test.ConstAdminID,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}
