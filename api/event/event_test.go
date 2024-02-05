package event

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/pkg/maputil"
)

func TestHandler_getEvent(t *testing.T) {
	app, teardownTestCase := test.API(t)
	New(app.Handler).Routes()
	app.AddRoute404()
	defer teardownTestCase(t)

	adminInfo := app.GetUserInfo("admin@werbot.net", "admin@werbot.net")
	userInfo := app.GetUserInfo("user@werbot.net", "user@werbot.net")

	testTable := []struct {
		name           string            // The name of the test
		method         string            // The HTTP method to use in our call
		path           string            // The URL path that is being requested
		statusCode     int               // The expected response status code
		body           map[string]any    // The expected response body
		requestBody    map[string]any    // The request body to sent with the request
		requestHeaders map[string]string // The headers that are being set for the request
		headers        map[string]string // The response headers we want to test on
	}{
		{
			name:       "unauthorized request",
			method:     http.MethodGet,
			path:       "/v1/event",
			statusCode: 401,
			body:       test.BodyUnauthorized,
		},
		{
			name:       "admin: request without parameters",
			method:     http.MethodGet,
			path:       "/v1/event",
			statusCode: 404,
			body:       test.BodyNotFound,
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},
		{
			name:       "admin: error displaying list events with invalid arguments",
			method:     http.MethodGet,
			path:       "/v1/event/profile/008feb1d-12f2-4bc3-97ff-c8d7fb9f7686?limit=abc",
			statusCode: 400,
			body:       test.BodyInvalidArgument,
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},
		{
			name:       "admin: error displaying list events with fake name",
			method:     http.MethodGet,
			path:       "/v1/event/abc/008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			statusCode: 400,
			body:       test.BodyInvalidArgument,
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},
		{
			name:       "admin: error displaying information about events with fake name",
			method:     http.MethodGet,
			path:       "/v1/event/abc/008feb1d-12f2-4bc3-97ff-c8d7fb9f7686/59fab0fa-8f0a-4065-8863-0dae40166015",
			statusCode: 400,
			body:       test.BodyInvalidArgument,
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},

		// profile event
		{
			name:       "admin: non-existent profile UUID",
			method:     http.MethodGet,
			path:       "/v1/event/profile/00000000-0000-0000-0000-000000000000",
			statusCode: 404,
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},
		{
			name:       "admin: list of all profile events",
			method:     http.MethodGet,
			path:       "/v1/event/profile/008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			statusCode: 200,
			body: map[string]any{
				"code":                float64(200),
				"result.total":        float64(2),
				"result.records.0.id": "59fab0fa-8f0a-4065-8863-0dae40166015",
				"result.records.1.id": "7c1bd7f9-2ef4-44c8-9756-0e85156ca58f",
			},
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},
		{
			name:       "admin: list of all profile events with limit",
			method:     http.MethodGet,
			path:       "/v1/event/profile/008feb1d-12f2-4bc3-97ff-c8d7fb9f7686?limit=1",
			statusCode: 200,
			body: map[string]any{
				"code":                float64(200),
				"result.total":        float64(2),
				"result.records.0.id": "59fab0fa-8f0a-4065-8863-0dae40166015",
				"result.records.1.id": nil,
			},
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},
		{
			name:       "admin: profile event information by UUID",
			method:     http.MethodGet,
			path:       "/v1/event/profile/008feb1d-12f2-4bc3-97ff-c8d7fb9f7686/59fab0fa-8f0a-4065-8863-0dae40166015",
			statusCode: 200,
			body: map[string]any{
				"code":             float64(200),
				"result.ip":        "2001:db8:85a3::8a2e:370:7334",
				"result.event":     float64(1),
				"result.meta_data": "e30=",
			},
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},
		{
			name:       "user: error displaying profile events not owned by the user",
			method:     http.MethodGet,
			path:       "/v1/event/profile/008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
			statusCode: 404,
			body:       test.BodyNotFound,
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + userInfo.Tokens.Access,
			},
		},
		{
			name:       "user: error displaying profile event info not owned by the user",
			method:     http.MethodGet,
			path:       "/v1/event/profile/008feb1d-12f2-4bc3-97ff-c8d7fb9f7686/59fab0fa-8f0a-4065-8863-0dae40166015",
			statusCode: 404,
			body:       test.BodyNotFound,
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + userInfo.Tokens.Access,
			},
		},

		// project event
		{
			name:       "admin: non-existent project UUID",
			method:     http.MethodGet,
			path:       "/v1/event/project/00000000-0000-0000-0000-000000000000",
			statusCode: 404,
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},
		{
			name:       "admin: list of all project events",
			method:     http.MethodGet,
			path:       "/v1/event/project/26060c68-5a06-4a57-b87a-be0f1e787157",
			statusCode: 200,
			body: map[string]any{
				"code":                float64(200),
				"result.total":        float64(2),
				"result.records.0.id": "163dee10-2a74-4436-9507-65a97a711ba8",
				"result.records.1.id": "9758b5ee-367d-4a70-965b-14a129cca4d7",
			},
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},
		{
			name:       "admin: list of all project events with limit",
			method:     http.MethodGet,
			path:       "/v1/event/project/26060c68-5a06-4a57-b87a-be0f1e787157?limit=1",
			statusCode: 200,
			body: map[string]any{
				"code":                float64(200),
				"result.total":        float64(2),
				"result.records.0.id": "163dee10-2a74-4436-9507-65a97a711ba8",
				"result.records.1.id": nil,
			},
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},
		{
			name:       "admin: project event information by UUID",
			method:     http.MethodGet,
			path:       "/v1/event/project/26060c68-5a06-4a57-b87a-be0f1e787157/163dee10-2a74-4436-9507-65a97a711ba8",
			statusCode: 200,
			body: map[string]any{
				"code":             float64(200),
				"result.ip":        "192.168.0.1",
				"result.event":     float64(1),
				"result.meta_data": "e30=",
			},
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},
		{
			name:       "user: error displaying project events not owned by the user",
			method:     http.MethodGet,
			path:       "/v1/event/project/26060c68-5a06-4a57-b87a-be0f1e787157",
			statusCode: 404,
			body:       test.BodyNotFound,
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + userInfo.Tokens.Access,
			},
		},
		{
			name:       "user: error displaying project event info not owned by the user",
			method:     http.MethodGet,
			path:       "/v1/event/project/26060c68-5a06-4a57-b87a-be0f1e787157/163dee10-2a74-4436-9507-65a97a711ba8",
			statusCode: 404,
			body:       test.BodyNotFound,
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + userInfo.Tokens.Access,
			},
		},

		// server event
		{
			name:       "admin: non-existent server UUID",
			method:     http.MethodGet,
			path:       "/v1/event/server/00000000-0000-0000-0000-000000000000",
			statusCode: 404,
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},
		{
			name:       "admin: list of all server events",
			method:     http.MethodGet,
			path:       "/v1/event/server/0c3a8869-6fc0-4666-bf60-15475473392a",
			statusCode: 200,
			body: map[string]any{
				"code":                float64(200),
				"result.total":        float64(11),
				"result.records.0.id": "dea438b3-ca64-45ad-80a6-51275730f078",
				"result.records.1.id": "a2ef053e-4124-487b-9e90-b8f249d49807",
			},
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},
		{
			name:       "admin: list of all server events with limit",
			method:     http.MethodGet,
			path:       "/v1/event/server/0c3a8869-6fc0-4666-bf60-15475473392a?limit=1",
			statusCode: 200,
			body: map[string]any{
				"code":                float64(200),
				"result.total":        float64(11),
				"result.records.0.id": "dea438b3-ca64-45ad-80a6-51275730f078",
				"result.records.1.id": nil,
			},
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},
		{
			name:       "admin: server event information by UUID",
			method:     http.MethodGet,
			path:       "/v1/event/server/0c3a8869-6fc0-4666-bf60-15475473392a/dea438b3-ca64-45ad-80a6-51275730f078",
			statusCode: 200,
			body: map[string]any{
				"code":             float64(200),
				"result.ip":        "192.168.1.1",
				"result.event":     float64(1),
				"result.meta_data": "e30=",
			},
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + adminInfo.Tokens.Access,
			},
		},
		{
			name:       "user: error displaying server events not owned by the user",
			method:     http.MethodGet,
			path:       "/v1/event/server/0c3a8869-6fc0-4666-bf60-15475473392a",
			statusCode: 404,
			body:       test.BodyNotFound,
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + userInfo.Tokens.Access,
			},
		},
		{
			name:       "user: error displaying server event info not owned by the user",
			method:     http.MethodGet,
			path:       "/v1/event/server/0c3a8869-6fc0-4666-bf60-15475473392a/dea438b3-ca64-45ad-80a6-51275730f078",
			statusCode: 404,
			body:       test.BodyNotFound,
			requestHeaders: map[string]string{
				`Authorization`: `Bearer ` + userInfo.Tokens.Access,
			},
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
				assert.Equal(t, statusCode, tc.statusCode)
			}

			// Headers
			for k, want := range tc.headers {
				headerValue := response.Header.Get(k)
				if headerValue != want {
					assert.Equal(t, headerValue, want, "Response header '"+k+"' was incorrect")
				}
			}

			// Response Body
			if len(tc.body) > 0 {
				body, _ := io.ReadAll(response.Body)
				var resp map[string]any
				json.Unmarshal(body, &resp)
				for key, want := range tc.body {
					val, _ := maputil.GetByPath(key, resp)
					if val != want {
						assert.Equal(t, val, want, "Body key ["+key+"] was incorrect")
					}
				}
			}
		})
	}
}
