package ping

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
)

func TestHandler_getPing(t *testing.T) {
	app, teardownTestCase := test.API(t)
	New(app.Handler).Routes()
	defer teardownTestCase(t)

	testTable := []struct {
		name           string                 // The name of the test
		method         string                 // The HTTP method to use in our call
		path           string                 // The URL path that is being requested
		statusCode     int                    // The expected response status code
		body           string                 // The expected response body, as string
		requestBody    map[string]interface{} // The request body to sent with the request
		requestHeaders map[string]string      // The headers that are being set for the request
		headers        map[string]string      // The response headers we want to test on
	}{
		{
			name:       "GET ping to get a answer",
			method:     http.MethodGet,
			path:       "/ping",
			statusCode: 200,
			body:       "pong",
		},
		{
			name:       "POST ping method not allowed",
			method:     http.MethodPost,
			path:       "/ping",
			statusCode: 405,
			body:       `Method Not Allowed`,
		},
	}

	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			// Create and send request
			rbody, _ := json.Marshal(tc.requestBody)
			request := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(rbody))
			request.Header.Add(`Content-Type`, `application/json`)

			// Request Headers
			for k, v := range tc.requestHeaders {
				request.Header.Add(k, v)
			}

			response, _ := app.App.Test(request)

			// Status Code
			statusCode := response.StatusCode
			if statusCode != tc.statusCode {
				t.Errorf("StatusCode was incorrect, got: %d, want: %d.", statusCode, tc.statusCode)
			}

			// Headers
			for k, want := range tc.headers {
				headerValue := response.Header.Get(k)
				if headerValue != want {
					t.Errorf("Response header '%s' was incorrect, got: '%s', want: '%s'", k, headerValue, want)
				}
			}

			// Response Body
			body, _ := io.ReadAll(response.Body)
			actual := string(body)
			if actual != tc.body {
				t.Errorf("Body was incorrect, got: %v, want: %v", actual, tc.body)
			}
		})
	}

}
