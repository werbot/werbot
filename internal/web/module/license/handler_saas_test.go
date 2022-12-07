//go:build saas

package license

import (
	"net/http"
	"testing"

	jsonpath "github.com/steinfletcher/apitest-jsonpath"
	"github.com/werbot/werbot/internal"
)

func TestHandler_getLicenseExpired(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		requestBody   any
		respondBody   func(*http.Response, *http.Request) error
		respondStatus int
	}{
		// Error validating body params
		{
			name: "getLicenseExpired_01",
			respondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.ErrValidateBodyParams).
				End(),
			respondStatus: http.StatusBadRequest,
		},

		// License expired - true, license invalid
		{
			name:        "getLicenseExpired_02",
			requestBody: map[string]string{"License": "LS0tLS1CRUdJTiBMSUNFTlNFIEtFWS0tLS0tCjlpOVlxdk1YZnpWQXB4RkVTODRUK0RtYW5mQUdpcmQ1R0Q5Zjl4SUxlN2N0Qlh3T1pVTmM2V1RKaktaaWhwMFYKTkF4V0hpblByWUJwZGlSN1plTm5DM3NpWTNWeklqb2lkMlZ5WW05MElpd2lkSGx3SWpvaVpXNTBaWEp3Y21segpaU0lzSW1saGRDSTZJakl3TWpJdE1ESXRNVFpVTWpBNk5USTZNelV1TnpneE5EVTFOemc1V2lJc0ltVjRjQ0k2CklqSXdNakl0TURNdE1UWlVNakE2TlRJNk16VXVOemd4TkRVMU56ZzVXaUlzSW1SaGRDSTZleUp6WlhKMlpYSnoKSWpveU1Dd2lZMjl0Y0dGdWFXVnpJam94TUN3aWRYTmxjbk1pT2pFd01Dd2liVzlrZFd4bGN5STZXeUp6ZFdOagpaWE56SWl3aVpYSnliM0lpTENKM1lYSnVhVzVuSWwxOWZRPT0KLS0tLS1FTkQgTElDRU5TRSBLRVktLS0tLQ=="},
			respondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "License expired").
				Equal(`$.result`, true).
				End(),
			respondStatus: http.StatusOK,
		},

		// License expired - false, license valid
		{
			name:        "getLicenseExpired_03",
			requestBody: map[string]string{"License": "LS0tLS1CRUdJTiBMSUNFTlNFIEtFWS0tLS0tCjRUOHpXak5pVTJuZGhCcmQ2NmxFclhpbnhoc3BVc1pzL3FvdDYyd0dOWWZoQlpCMWx3V1RhaUk2dXVLT2hPSDkKZE9PbkEzZ3ZvZzFiN29ZTXZDc1dDM3NpYVhOeklqb2lWMlZ5WW05MFh6SXdNakl3TlRJMk1qRXhOakUxTENCSgpibU11SWl3aVkzVnpJam9pTXpRMlpqUmlORGt0WmprME1DMDBOek00TFRrNU5XRXRNekV3TW1JNFpUZGtNbUpsCklpd2ljM1ZpSWpvaU5qRTRZV1ZrTW1ZdE1tVTVOaTAwTkRZeUxUaGpOekF0TldJNU1qWTVNRGN3WVRaaklpd2kKZEhsd0lqb2lSVzUwWlhKd2NtbHpaU0IwY21saGJDSXNJbWx3Y3lJNklqRXlOeTR3TGpBdU1TSXNJbWxoZENJNgpJakl3TWpJdE1EVXRNalpVTWpFNk1UWTZNVFV1TURnd09UQTNPVEU0V2lJc0ltVjRjQ0k2SWpJd01qVXRNREl0Ck1UaFVNakU2TVRZNk1UVXVNRGd3T1RBM09URTRXaUlzSW1SaGRDSTZleUpqYjIxd1lXNXBaWE1pT2pVc0luTmwKY25abGNuTWlPakl3TUN3aWRYTmxjbk1pT2pJd0xDSnRiMlIxYkdWeklqcGJJbk4xWTJObGMzTWlMQ0psY25KdgpjaUlzSW5kaGNtNXBibWNpWFgxOQotLS0tLUVORCBMSUNFTlNFIEtFWS0tLS0tCg=="},
			respondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "License expired").
				Equal(`$.result`, false).
				End(),
			respondStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			apiTest().
				//Debug().
				HandlerFunc(testHandler.Handler).
				Get("/v1/license/expired").
				JSON(tc.requestBody).
				Expect(t).
				Assert(tc.respondBody).
				Status(tc.respondStatus).
				End()
		})
	}
}

func TestHandler_postLicense(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		requestBody   any
		RequestToken  string
		respondBody   func(*http.Response, *http.Request) error
		respondStatus int
	}{
		// ROLE_ADMIN - administrator-only params
		{
			name: "ROLE_ADMIN_postLicense_01",
			requestBody: map[string]any{
				"ip":         "127.0.0.1",
				"customer":   "8fdc79f2-dddd-11ec-9d64-0242ac120002",
				"subscriber": "331aa86d-740b-4f6a-b7ba-07cf3bf041e3",
			},
			RequestToken: adminInfo.Tokens.Access,
			respondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "License key").
				End(),
			respondStatus: http.StatusOK,
		},

		// Error validating body params
		{
			name: "postLicense_01",
			respondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.ErrValidateBodyParams).
				End(),
			respondStatus: http.StatusBadRequest,
		},

		// Error validating ip address
		{
			name: "postLicense_02",
			requestBody: map[string]any{
				"ip": "257.0.0.0",
			},
			respondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.ErrValidateBodyParams).
				End(),
			respondStatus: http.StatusBadRequest,
		},

		// Error invalid body params
		{
			name: "postLicense_03",
			requestBody: map[string]any{
				"ip":         "257.0.0.0",
				"customer":   "8fdc79f2-dddd-11ec-9d64-0242ac120002+",
				"subscriber": "331aa86d-740b-4f6a-b7ba-07cf3bf041e3+",
				"token":      "lfuD9PL9NfRMIKA1tj8VFLsPbORALIBY+",
			},
			respondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.ErrValidateBodyParams).
				Equal(`$.result.ip`, "Ip must be a valid IP address").
				Equal(`$.result.customer`, "Customer must be a valid UUID").
				Equal(`$.result.subscriber`, "Subscriber must be a valid UUID").
				Equal(`$.result.token`, "Token can only contain alphanumeric characters").
				End(),
			respondStatus: http.StatusBadRequest,
		},

		// Generate new trial license
		{
			name: "postLicense_04",
			requestBody: map[string]any{
				"ip": "254.254.254.254",
			},
			respondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "License key").
				End(),
			respondStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			apiTest().
				//Debug().
				HandlerFunc(testHandler.Handler).
				Post("/v1/license").
				JSON(tc.requestBody).
				Header("Authorization", "Bearer "+tc.RequestToken).
				Expect(t).
				Assert(tc.respondBody).
				Status(tc.respondStatus).
				End()
		})
	}
}
