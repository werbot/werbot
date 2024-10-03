package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/utils/maputil"
)

//
// For information:
//
// 1. The following template is used to designate test names:
// test0_## - for tests that do not require authorization
// test1_## - for tests where ADMIN is authorized
// test2_## - for tests where USER is authorized
//

type (
	BodyTable    map[string]any
	ProtoMessage protoreflect.ProtoMessage
)

// PathGluing is ...
func PathGluing(path ...string) string {
	if len(path) == 0 {
		return ""
	}
	return strings.Join(path, "/")
}

// RunCaseAPITests is ...
func RunCaseAPITests(t *testing.T, app *APIHandler, testTable []APITable) {
	checkValue := func(expected, actual any, contextKey string) {
		if expected == "*" {
			if reflect.TypeOf(actual).String() != "bool" {
				assert.NotEmpty(t, actual, "Body key [%s] was incorrect", contextKey)
			}
		} else {
			assert.Equal(t, expected, actual, "Body key [%s] was incorrect", contextKey)
		}
	}

	for _, tc := range testTable {
		if tc.PreWorkHook != nil {
			tc.PreWorkHook()
		}

		t.Run(tc.Name, func(t *testing.T) {
			rbody, _ := json.Marshal(tc.RequestBody)
			request := httptest.NewRequest(tc.Method, tc.Path, bytes.NewReader(rbody))

			switch tc.Method {
			case http.MethodPost, http.MethodPatch:
				request.Header.Add("Content-Type", "application/json")
			}

			for k, v := range tc.RequestHeaders {
				request.Header.Add(k, v)
			}

			response, _ := app.App.Test(request)
			defer response.Body.Close()

			assert.Equal(t, tc.StatusCode, response.StatusCode)

			for k, want := range tc.Headers {
				headerValue := response.Header.Get(k)
				assert.Equal(t, want, headerValue, "Response header '%s' was incorrect", k)
			}

			if len(tc.Body) > 0 {
				body, _ := io.ReadAll(response.Body)
				var resp BodyTable
				json.Unmarshal(body, &resp)

				for key, want := range tc.Body {
					val, _ := maputil.GetByPath(key, resp)

					switch v := want.(type) {
					case nil, float64, bool, string:
						checkValue(want, val, key)
					case map[string]any:
						nestedMap, ok := want.(map[string]any)
						if !ok {
							assert.Errorf(t, nil, "Body key [%s] has an unknown type [%T]", key, v)
							continue
						}
						for keyTable, wantTable := range nestedMap {
							valTable, _ := maputil.GetByPath(keyTable, resp["result"].(map[string]any))
							checkValue(wantTable, valTable, key)
						}
					default:
						assert.Errorf(t, nil, "Body key [%s] has an unknown type [%T]", key, v)
					}
				}
			}
		})
	}
}

// RunCaseGRPCTests is ...
func RunCaseGRPCTests(t *testing.T, handler func(context.Context, ProtoMessage) (ProtoMessage, error), testTable []GRPCTable) {
	checkValue := func(expected, actual any, contextKey string) {
		if expected == "*" {
			assert.NotEmpty(t, actual, "Key [%s] was incorrect", contextKey)
		} else {
			assert.Equal(t, expected, actual, "Key [%s] was incorrect", contextKey)
		}
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.PreWorkHook != nil {
				tt.PreWorkHook()
			}

			response, err := handler(context.Background(), tt.Request)

			if tt.Debug {
				debugResponse, _ := protojson.MarshalOptions{
					UseEnumNumbers: true,
					UseProtoNames:  true,
					Multiline:      true,
				}.Marshal(response)
				t.Logf("\nDebug data: %s", debugResponse)
			}

			if err != nil {
				dataError := trace.ParseError(err)

				if tt.Error.Code > 0 {
					assert.Equal(t, tt.Error.Code.String(), dataError.Code.String())
				}

				switch m := tt.Error.Message.(type) {
				case string:
					assert.Equal(t, m, dataError.Message, "Error was incorrect")
				case map[string]any:
					errorMap := make(map[string]any)
					lines := strings.Split(dataError.Message, "\n")
					for _, line := range lines {
						if colonIndex := strings.IndexByte(line, ':'); colonIndex != -1 {
							key := strings.TrimSpace(line[:colonIndex])
							value := strings.TrimSpace(line[colonIndex+1:])
							errorMap[key] = value
						}
					}
					for key, want := range m {
						val, _ := maputil.GetByPath(key, errorMap)
						checkValue(want, val, key)
					}
				default:
					assert.Errorf(t, nil, "Error key has an unknown type [%T]", m)
				}
			}

			protoResp, _ := convertProtoToBodyTable(response)
			for key, want := range tt.Response {
				val, _ := maputil.GetByPath(key, protoResp)
				checkValue(want, val, key)
			}
		})
	}
}
