package user

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_users(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathUsers, "list"),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathUsers, "list"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":         float64(200),
				"message":      "Users",
				"result.total": float64(22),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathUsers, "list") + "?limit=2&offset=0",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":           float64(200),
				"message":        "Users",
				"result.total":   float64(22),
				"result.users.0": "*",
				"result.users.1": "*",
				"result.users.2": nil,
			},
			RequestHeaders: adminHeader,
		},
		{ // USER:
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathUsers, "list"),
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

func TestHandler_user(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathUsers,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       pathUsers,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                  float64(200),
				"message":               "User",
				"result.user_id":        test.ConstAdminID,
				"result.alias":          "admin",
				"result.name":           "Penny",
				"result.surname":        "Hoyle",
				"result.email":          "admin@werbot.net",
				"result.active":         true,
				"result.confirmed":      true,
				"result.role":           float64(3),
				"result.locked_at":      nil,
				"result.archived_at":    nil,
				"result.updated_at":     "*",
				"result.created_at":     "*",
				"result.projects_count": float64(11),
				"result.schemes_count":  float64(79),
				"result.keys_count":     float64(12),
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       pathUsers + "?user_id=" + test.ConstFakeID,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       pathUsers + "?user_id=" + test.ConstUserID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                  float64(200),
				"message":               "User",
				"result.user_id":        test.ConstUserID,
				"result.alias":          "user",
				"result.name":           "Carly",
				"result.surname":        "Bender",
				"result.email":          "user@werbot.net",
				"result.active":         true,
				"result.confirmed":      true,
				"result.role":           float64(1),
				"result.locked_at":      nil,
				"result.archived_at":    nil,
				"result.updated_at":     nil,
				"result.created_at":     "*",
				"result.projects_count": float64(3),
				"result.schemes_count":  float64(68),
				"result.keys_count":     float64(2),
			},
			RequestHeaders: adminHeader,
		},

		{ // USER:
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathUsers),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                  float64(200),
				"message":               "User",
				"result.user_id":        test.ConstUserID,
				"result.alias":          "user",
				"result.name":           "Carly",
				"result.surname":        "Bender",
				"result.email":          "user@werbot.net",
				"result.active":         true,
				"result.confirmed":      true,
				"result.role":           float64(1),
				"result.locked_at":      nil,
				"result.archived_at":    nil,
				"result.updated_at":     nil,
				"result.created_at":     "*",
				"result.projects_count": float64(3),
				"result.schemes_count":  float64(68),
				"result.keys_count":     float64(2),
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_02",
			Method:     http.MethodGet,
			Path:       pathUsers + "?user_id=" + test.ConstFakeID,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                  float64(200),
				"message":               "User",
				"result.user_id":        test.ConstUserID,
				"result.alias":          "user",
				"result.name":           "Carly",
				"result.surname":        "Bender",
				"result.email":          "user@werbot.net",
				"result.active":         true,
				"result.confirmed":      true,
				"result.role":           float64(1),
				"result.locked_at":      nil,
				"result.archived_at":    nil,
				"result.updated_at":     nil,
				"result.created_at":     "*",
				"result.projects_count": float64(3),
				"result.schemes_count":  float64(68),
				"result.keys_count":     float64(2),
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_addUser(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPost,
			Path:       pathUsers,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodPost,
			Path:       pathUsers,
			StatusCode: 400,
			Body: test.BodyTable{
				"code":            float64(400),
				"message":         "Bad Request",
				"result.alias":    "value is required",
				"result.email":    "value is required",
				"result.name":     "value is required",
				"result.surname":  "value is required",
				"result.password": "value is required",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodPost,
			Path:       pathUsers,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"alias":    "az",
				"email":    "az",
				"name":     "az",
				"surname":  "az",
				"password": "az",
			},
			Body: test.BodyTable{
				"code":            float64(400),
				"message":         "Bad Request",
				"result.alias":    "value length must be at least 3 characters",
				"result.email":    "must be a valid email",
				"result.name":     "value length must be at least 3 characters",
				"result.surname":  "value length must be at least 3 characters",
				"result.password": "value length must be at least 8 characters",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: double
			Name:       "test1_03",
			Method:     http.MethodPost,
			Path:       pathUsers,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"alias":    "alias",
				"email":    "user@werbot.net",
				"name":     "Name",
				"surname":  "Surname",
				"password": "password",
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  "Failed to add",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodPost,
			Path:       pathUsers,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"alias":    "alias",
				"email":    "user@mail.com",
				"name":     "Name",
				"surname":  "Surname",
				"password": "password",
			},
			Body: test.BodyTable{
				"code":           float64(200),
				"message":        "User added",
				"result.user_id": "*",
			},
			RequestHeaders: adminHeader,
		},
		{ // USER:
			Name:       "test2_01",
			Method:     http.MethodPost,
			Path:       pathUsers,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"alias":    "alias",
				"email":    "user@mail.com",
				"name":     "Name",
				"surname":  "Surname",
				"password": "password",
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

func TestHandler_updateUser(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"test": "az",
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"alias": "az",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.alias": "value length must be at least 3 characters",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"email": "az",
			},
			Body: test.BodyTable{
				"code":         float64(400),
				"message":      "Bad Request",
				"result.email": "must be a valid email",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_05",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"name": "az",
			},
			Body: test.BodyTable{
				"code":        float64(400),
				"message":     "Bad Request",
				"result.name": "value length must be at least 3 characters",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_06",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"surname": "az",
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.surname": "value length must be at least 3 characters",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_07",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"user_id": test.ConstFakeID,
				"alias":   "alias",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_08",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"email": "user@email.com",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_09",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"name": "name",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_10",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"surname": "surname",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_11",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"confirmed": true,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_12",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"active": true,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_13",
			Method:     http.MethodPatch,
			Path:       pathUsers + "?user_id=" + test.ConstFakeID,
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"alias": "alias",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_14",
			Method:     http.MethodPatch,
			Path:       pathUsers + "?user_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"alias": "alias",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_15",
			Method:     http.MethodPatch,
			Path:       pathUsers + "?user_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"email": "user@email.com",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_16",
			Method:     http.MethodPatch,
			Path:       pathUsers + "?user_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"name": "name",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_17",
			Method:     http.MethodPatch,
			Path:       pathUsers + "?user_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"surname": "surname",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_18",
			Method:     http.MethodPatch,
			Path:       pathUsers + "?user_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"confirmed": true,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User updated",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_19",
			Method:     http.MethodPatch,
			Path:       pathUsers + "?user_id=" + test.ConstUserID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"active": true,
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User updated",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER:
			Name:       "test2_1",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"name": "New Name",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User updated",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_2",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"surname": "New Surname",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User updated",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_3",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"email": "new@mail.com",
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_4",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"confirmed": true,
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_5",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"active": true,
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: userHeader,
		},
		{ // USER:
			Name:       "test2_6",
			Method:     http.MethodPatch,
			Path:       pathUsers,
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"archive": true,
			},
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.setting": "exactly one field is required in oneof",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: ignored user_id
			Name:       "test2_7",
			Method:     http.MethodPatch,
			Path:       pathUsers + "?user_id=" + test.ConstFakeID,
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"name": "New Name",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User updated",
			},
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_deleteUser(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request: step 1
			Name:       "test0_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathUsers, "delete"),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // unauthorized request: step 2
			Name:       "test0_02",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathUsers, "delete"),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: step 1
			Name:       "test1_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathUsers, "delete"),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":           float64(400),
				"message":        "Bad Request",
				"result.request": "exactly one field is required in oneof",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: step 1, bad password
			Name:       "test1_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathUsers, "delete"),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"password": "password",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: step 1, real password
			// The administrator cannot be deleted!!!
			Name:       "test1_03",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathUsers, "delete"),
			StatusCode: 404,
			RequestBody: test.BodyTable{
				"password": "admin@werbot.com",
			},
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN: step 1, real password for delete oth
			// The administrator cannot be deleted!!!
			Name:       "test1_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathUsers, "delete", "3c818d7c-72f3-4518-8eaa-755585192f21"),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
			},
			RequestHeaders: adminHeader,
		},

		{ // USER: step 1, bad password
			Name:       "test2_01",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathUsers, "delete"),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"password": "password",
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  "Password is not valid",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: step 1, real password
			Name:       "test2_02",
			Method:     http.MethodPost,
			Path:       test.PathGluing(pathUsers, "delete"),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"password": "user@werbot.net",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Request for delete",
				"result":  "An email with instructions to delete your profile has been sent to your email",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: step 2, bad token
			Name:       "test2_03",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathUsers, "delete", test.ConstFakeID),
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: step 2, real token
			Name:       "test2_04",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathUsers, "delete", "8c7c9b35-1c3e-4679-ab2d-3e176a2b73d9"),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "User deleted",
			},
			RequestHeaders: userHeader,
		},
		{ // USER: step 2, disabled token
			Name:       "test2_05",
			Method:     http.MethodDelete,
			Path:       test.PathGluing(pathUsers, "delete", "0fcd88b3-8abb-4eb1-b96c-e0e49964cbca"),
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

func TestHandler_updatePassword(t *testing.T) {
	t.Setenv("PASSWORD_HASH_COST", "1")

	app, teardownTestCase, adminHeader, _ := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathUsers, "password"),
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathUsers, "password"),
			StatusCode: 400,
			Body: test.BodyTable{
				"code":                float64(400),
				"message":             "Bad Request",
				"result.new_password": "value is required",
				"result.old_password": "value is required",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathUsers, "password"),
			StatusCode: 400,
			RequestBody: test.BodyTable{
				"old_password": "12345678",
				"new_password": "12345678",
			},
			Body: test.BodyTable{
				"code":    float64(400),
				"message": "Bad Request",
				"result":  "Password is not valid",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodPatch,
			Path:       test.PathGluing(pathUsers, "password"),
			StatusCode: 200,
			RequestBody: test.BodyTable{
				"old_password": "admin@werbot.net",
				"new_password": "123456789",
			},
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Password updated",
			},
			RequestHeaders: adminHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}
