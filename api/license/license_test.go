package license

import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/pkg/utils/fsutil"
)

func TestHandler_licenseInfo(t *testing.T) {
	app, teardownTestCase, adminHeader, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // unauthorized request
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       pathLicenseInfo,
			StatusCode: 401,
			Body:       test.BodyUnauthorized,
		},
		{ // ADMIN: license info
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       pathLicenseInfo,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                    float64(200),
				"message":                 "License",
				"result.customer":         "Mr. Robot",
				"result.expired":          true,
				"result.expires_at":       "*",
				"result.issued":           "free",
				"result.issued_at":        "*",
				"result.limits.Companies": float64(99),
				"result.limits.Schemes":   float64(99),
				"result.limits.Users":     float64(99),
				"result.modules.0":        "module1",
				"result.modules.1":        "module2",
				"result.modules.2":        "module3",
			},
			RequestHeaders: adminHeader,
		},
		{ // USER: license info
			Name:           "test2_01",
			Method:         http.MethodGet,
			Path:           pathLicenseInfo,
			StatusCode:     404,
			Body:           test.BodyNotFound,
			RequestHeaders: userHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_licenseInfo_ee(t *testing.T) {
	app, teardownTestCase, adminHeader, _ := setupTest(t)
	defer teardownTestCase(t)

	pubKeyOk := string(fsutil.MustReadFile("../../fixtures/licenses/publicKey_ok.key"))
	t.Setenv("LICENSE_KEY_PUBLIC", pubKeyOk)
	t.Setenv("LICENSE_FILE", "../../fixtures/licenses/license_ok.key")

	testTable := []test.APITable{
		{ // ADMIN: EE license info
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       pathLicenseInfo,
			StatusCode: 200,
			Body: test.BodyTable{
				"code":                    float64(200),
				"message":                 "License",
				"result.customer":         "8ED96811-1804-4A13-9CE7-05874869A1CF",
				"result.expires_at":       "*",
				"result.issued":           "Werbot, Inc.",
				"result.issued_at":        "*",
				"result.subscriber":       "EED1CA19-4DC5-4376-83F5-61077B501961",
				"result.type":             "Enterprise",
				"result.limits.Companies": float64(99),
				"result.limits.Schemes":   float64(99),
				"result.limits.Users":     float64(99),
				"result.modules.0":        "module1",
				"result.modules.1":        "module2",
				"result.modules.2":        "module3",
			},
			RequestHeaders: adminHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}

func TestHandler_licenseInfo_ee_broken(t *testing.T) {
	app, teardownTestCase, adminHeader, _ := setupTest(t)
	defer teardownTestCase(t)

	t.Setenv("LICENSE_FILE", "../../fixtures/licenses/license_exp.key")
	t.Setenv("LICENSE_KEY_PUBLIC", "../../fixtures/licenses/publicKey_ok.key")

	testTable := []test.APITable{
		{ // ADMIN: license info
			Name:       "test1_01",
			Method:     http.MethodGet,
			Path:       pathLicenseInfo,
			StatusCode: 404,
			Body: test.BodyTable{
				"code":    float64(404),
				"message": "Not Found",
				"result":  "The license key has a broken",
			},
			RequestHeaders: adminHeader,
		},
	}

	test.RunCaseAPITests(t, app, testTable)
}
