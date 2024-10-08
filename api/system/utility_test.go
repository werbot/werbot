package system

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_myIP(t *testing.T) {
	app, teardownTestCase, adminHeader, _ := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathSystemMyIP,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "IP",
				"result":  "0.0.0.0",
			},
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       pathSystemMyIP,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "IP",
				"result":  "0.0.0.0",
			},
			RequestHeaders: adminHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_countries(t *testing.T) {
	app, teardownTestCase, adminHeader, _ := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathSystemVersion,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN:
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       pathSystemCountries,
			StatusCode: 400,
			Body: test.BodyTable{
				"code":        float64(400),
				"message":     "Bad Request",
				"result.name": "value is required",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_02",
			Method:     http.MethodGet,
			Path:       pathSystemCountries + "?name=be",
			StatusCode: 400,
			Body: test.BodyTable{
				"code":        float64(400),
				"message":     "Bad Request",
				"result.name": "value length must be at least 3 characters",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_03",
			Method:     http.MethodGet,
			Path:       pathSystemCountries + "?name=bel",
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                    float64(200),
				"message":                 "Countries",
				"result.countries.0.code": "BY",
				"result.countries.0.name": "Belarus",
				"result.countries.1.code": "BE",
				"result.countries.1.name": "Belgium",
				"result.countries.2.code": "BZ",
				"result.countries.2.name": "Belize",
			},
			RequestHeaders: adminHeader,
		},
		{ // ADMIN:
			Name:       "test1_04",
			Method:     http.MethodGet,
			Path:       pathSystemCountries + "?name=test",
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "Not found",
			},
			RequestHeaders: adminHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}
