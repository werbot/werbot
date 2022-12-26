package key

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"

	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/crypto"
	"github.com/werbot/werbot/internal/tests"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/api/proto/key"
	pb_key "github.com/werbot/werbot/api/proto/key"
	pb_user "github.com/werbot/werbot/api/proto/user"
)

var (
	testHandler *tests.TestHandler
	adminInfo   *tests.UserInfo
	userInfo    *tests.UserInfo
)

func init() {
	testHandler = tests.InitTestServer("../../../../.env")
	authMiddleware := middleware.Auth(testHandler.Cache).Execute()
	webHandler := &web.Handler{
		App:  testHandler.App,
		Grpc: testHandler.GRPC,
		Auth: authMiddleware,
	}

	New(webHandler).Routes()    // add test module handler
	testHandler.FinishHandler() // init finale handler for apitest

	adminInfo = testHandler.GetUserInfo(&pb_user.SignIn_Request{
		Email:    "test-admin@werbot.net",
		Password: "test-admin@werbot.net",
	})

	userInfo = testHandler.GetUserInfo(&pb_user.SignIn_Request{
		Email:    "test-user@werbot.net",
		Password: "test-user@werbot.net",
	})
}

func apiTest() *apitest.APITest {
	return apitest.New().
		//Debug().
		HandlerFunc(testHandler.Handler)
}

func newKey() string {
	key, err := crypto.NewSSHKey("KEY_TYPE_ED25519")
	if err != nil {
		return crypto.MsgFailedCreatingSSHKey
	}
	return string(key.PublicKey)
}

func Test_getKey(t *testing.T) {
	t.Parallel()
	testCases := map[string][]tests.TestCase{}

	testCases["ROLE_USER_UNSPECIFIED"] = []tests.TestCase{
		{
			Name:         "Without parameters",
			RequestParam: map[string]string{},
			RequestUser:  &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},
		{
			Name: "User keys your user_id",
			RequestParam: map[string]string{
				"user_id": adminInfo.UserID,
			},
			RequestUser: &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},
		{
			Name: "Send your key_id", // for user test-admin@werbot.net
			RequestParam: map[string]string{
				"key_id": adminInfo.UserID,
			},
			RequestUser: &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},
	}

	testCases["ROLE_ADMIN"] = []tests.TestCase{
		{
			Name:         "Without parameters",
			RequestParam: map[string]string{},
			RequestUser:  adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "user keys").
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "User keys your user_id", // for user test-admin@werbot.net
			RequestParam: map[string]string{
				"user_id": adminInfo.UserID,
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "user keys").
				Equal(`$.result.total`, float64(3)).
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "Send your key_id", // for user test-admin@werbot.net
			RequestParam: map[string]string{
				"key_id": "d8ac2125-1770-4fd5-94a8-b5d83cde47aa",
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "key information").
				Equal(`$.result.user_id`, adminInfo.UserID).
				Equal(`$.result.title`, "test_key 1").
				Equal(`$.result.fingerprint`, "fb:22:c9:13:5f:37:09:14:26:f0:84:9b:35:fd:a7:ba").
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "All keys another user_id", // for user test-user@werbot.net
			RequestParam: map[string]string{
				"user_id": userInfo.UserID,
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "user keys").
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "Show key information by another user_id", // for user test-admin@werbot.net
			RequestParam: map[string]string{
				"key_id":  "1a57bee5-1dca-4965-b6c2-bc60ba5526af",
				"user_id": adminInfo.UserID,
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgNotFound).
				End(),
			RespondStatus: http.StatusNotFound,
		},
		{
			Name: "Send non-existent user_id",
			RequestParam: map[string]string{
				"user_id": "00000000-0000-0000-0000-000000000000",
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgNotFound).
				End(),
			RespondStatus: http.StatusNotFound,
		},
		{
			Name: "Send non-existent key_id",
			RequestParam: map[string]string{
				"key_id":  "00000000-0000-0000-0000-000000000000",
				"user_id": "e9a1f437-8f32-4463-9f89-a886a623febc",
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgNotFound).
				End(),
			RespondStatus: http.StatusNotFound,
		},
		{
			Name: "Invalid user_id parameter",
			RequestParam: map[string]string{
				"user_id": "00000000-0000",
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateParams).
				Equal(`$.result.userid`, "userId must be a valid UUID").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name: "Invalid key_id parameter",
			RequestParam: map[string]string{
				"key_id": "00000000-0000",
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateParams).
				Equal(`$.result.keyid`, "keyId must be a valid UUID").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name: "Invalid key_id and user_id parameter",
			RequestParam: map[string]string{
				"key_id":  "00000000-0000",
				"user_id": "00000000-0000",
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateParams).
				Equal(`$.result.userid`, "userId must be a valid UUID").
				Equal(`$.result.keyid`, "keyId must be a valid UUID").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
	}

	testCases["ROLE_USER"] = []tests.TestCase{
		{
			Name:         "Without parameters",
			RequestParam: map[string]string{},
			RequestUser:  userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "user keys").
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "Send your user_id", // for user test-user@werbot.net
			RequestParam: map[string]string{
				"user_id": userInfo.UserID,
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "user keys").
				Equal(`$.result.total`, float64(2)).
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "Send another user_id", // for user test-admin@werbot.net, this parameters ignoring for this roles
			RequestParam: map[string]string{
				"user_id": adminInfo.UserID,
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "user keys").
				Equal(`$.result.total`, float64(2)).
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "Invalid user_id parameter",
			RequestParam: map[string]string{
				"user_id": "00000000-0000",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateParams).
				Equal(`$.result.userid`, "userId must be a valid UUID").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name: "Invalid key_id parameter",
			RequestParam: map[string]string{
				"key_id": "00000000-0000",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateParams).
				Equal(`$.result.keyid`, "keyId must be a valid UUID").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name: "Invalid key_id and user_id parameter",
			RequestParam: map[string]string{
				"key_id":  "00000000-0000",
				"user_id": "00000000-0000",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateParams).
				Equal(`$.result.userid`, "userId must be a valid UUID").
				Equal(`$.result.keyid`, "keyId must be a valid UUID").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
	}

	for role, rtc := range testCases {
		t.Run(role, func(t *testing.T) {
			for _, tc := range rtc {
				t.Run(tc.Name, func(t *testing.T) {
					apiTest().
						Get("/v1/keys").
						QueryParams(tc.RequestParam.(map[string]string)).
						Header("Authorization", "Bearer "+tc.RequestUser.Tokens.Access).
						Expect(t).
						Assert(tc.RespondBody).
						Status(tc.RespondStatus).
						End()
				})
			}
		})
	}
}

func Test_addKey(t *testing.T) {
	t.Parallel()
	testCases := map[string][]tests.TestCase{}

	testCases["ROLE_USER_UNSPECIFIED"] = []tests.TestCase{
		{
			Name:        "Without parameters",
			RequestBody: pb_key.AddPublicKey_Request{},
			RequestUser: &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},
	}

	testCases["ROLE_ADMIN"] = []tests.TestCase{
		{
			Name:        "Without parameters",
			RequestBody: pb_key.AddPublicKey_Request{},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateBody).
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name: "Add key by your user_id", // for user test-admin@werbot.net
			RequestBody: pb_key.AddPublicKey_Request{
				UserId: adminInfo.UserID,
				Key:    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFQmBo7P94hOf00dl3DKfYNcLz2Sd1WRs6SHZE6rFQlx",
				Title:  "test1",
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "new key added").
				End(),
			RespondStatus: http.StatusOK,
		},
	}

	testCases["ROLE_USER"] = []tests.TestCase{
		{
			Name:        "Without parameters",
			RequestBody: pb_key.AddPublicKey_Request{},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateBody).
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name: "Adding a key to your profile",
			RequestBody: pb_key.AddPublicKey_Request{
				Key:   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQCPl/FmQtusakH1dqqB2jwbZ5gI0BEhjpLsSx/NfnjF+6Nmd1+lk533pyDod3KJYBzr/TjgOLp7jTLw+GcrczjqBLboUSXb564eokApOSrEBvko/MGtcIaJ5RGtenljPtHgbt3N/ldeiGXgUfImpDkYDXY9RpG5d0CN7YtgouIqIQ==",
				Title: "test1",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "new key added").
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "Add key for another user_id", // for user test-admin@werbot.net, this parameters ignoring for this roles
			RequestBody: pb_key.AddPublicKey_Request{
				UserId: adminInfo.UserID,
				Key:    "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQCPl/FmQtusakH1dqqB2jwbZ5gI0BEhjpLsSx/NfnjF+6Nmd1+lk533pyDod3KJYBzr/TjgOLp7jTLw+GcrczjqBLboUSXb564eokApOSrEBvko/MGtcIaJ5RGtenljPtHgbt3N/ldeiGXgUfImpDkYDXY9RpG5d0CN7YtgouIqIQ==",
				Title:  "test1",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "new key added").
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "Invalid key parameter",
			RequestBody: pb_key.AddPublicKey_Request{
				Key:   "ssh-rsa AsSx/NfnjF+OSrEBveiGXgUfImpDkYDXY9RpG5d0CN7YtgouIqIQ==",
				Title: "test1",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, "The public key has a broken structure").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name: "Invalid title parameter",
			RequestBody: pb_key.AddPublicKey_Request{
				Key: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQCPl/FmQtusakH1dqqB2jwbZ5gI0BEhjpLsSx/NfnjF+6Nmd1+lk533pyDod3KJYBzr/TjgOLp7jTLw+GcrczjqBLboUSXb564eokApOSrEBvko/MGtcIaJ5RGtenljPtHgbt3N/ldeiGXgUfImpDkYDXY9RpG5d0CN7YtgouIqIQ==",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateBody).
				Equal(`$.result.title`, "Title is a required field").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name: "Use another user_id when adding a new key",
			RequestBody: pb_key.AddPublicKey_Request{
				UserId: adminInfo.UserID,
				Key:    "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQCPl/FmQtusakH1dqqB2jwbZ5gI0BEhjpLsSx/NfnjF+6Nmd1+lk533pyDod3KJYBzr/TjgOLp7jTLw+GcrczjqBLboUSXb564eokApOSrEBvko/MGtcIaJ5RGtenljPtHgbt3N/ldeiGXgUfImpDkYDXY9RpG5d0CN7YtgouIqIQ==",
				Title:  "test1",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "new key added").
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "Object already exists",
			RequestBody: pb_key.AddPublicKey_Request{
				Key:   "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGWl2aY8FicEWNAlrQ+DwmhonSuhU8SsXJErdO9WpPKN",
				Title: "test1",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, "Object already exists").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
	}

	for role, rtc := range testCases {
		t.Run(role, func(t *testing.T) {
			for _, tc := range rtc {
				t.Run(tc.Name, func(t *testing.T) {
					resp := apiTest().
						Post("/v1/keys").
						JSON(tc.RequestBody).
						Header("Authorization", "Bearer "+tc.RequestUser.Tokens.Access).
						Expect(t).
						Assert(tc.RespondBody).
						Status(tc.RespondStatus).
						End()

					// delete added project
					data := map[string]pb_key.AddPublicKey_Response{}
					json.NewDecoder(resp.Response.Body).Decode(&data)
					if data["result"].KeyId != "" {
						ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
						defer cancel()
						rClient := pb_key.NewKeyHandlersClient(testHandler.GRPC.Client)

						rClient.DeletePublicKey(ctx, &pb_key.DeletePublicKey_Request{
							UserId: tc.RequestUser.UserID,
							KeyId:  data["result"].KeyId,
						})
					}
				})
			}
		})
	}
}

func Test_patchKey(t *testing.T) {
	t.Parallel()
	testCases := map[string][]tests.TestCase{}

	testCases["ROLE_USER_UNSPECIFIED"] = []tests.TestCase{
		{
			Name:        "Without parameters",
			RequestBody: pb_key.AddPublicKey_Request{},
			RequestUser: &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},
	}

	testCases["ROLE_ADMIN"] = []tests.TestCase{
		{
			Name:        "Without parameters",
			RequestBody: pb_key.UpdatePublicKey_Request{},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateBody).
				Equal(`$.result.key`, "key is a required field").
				Equal(`$.result.keyid`, "keyId is a required field").
				Equal(`$.result.title`, "title is a required field").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name: "Parameter key is filled",
			RequestBody: pb_key.UpdatePublicKey_Request{
				KeyId: "6437da7e-c9a4-463d-8b4d-2a7d1f730875",
				Key:   "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGWl2aY8FicEWNAlrQ+DwmhonSuhU8SsXJErdO9WpPKN",
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateBody).
				Equal(`$.result.title`, "title is a required field").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name: "Key has already been added",
			RequestBody: pb_key.UpdatePublicKey_Request{
				KeyId: "6437da7e-c9a4-463d-8b4d-2a7d1f730875",
				Key:   "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGWl2aY8FicEWNAlrQ+DwmhonSuhU8SsXJErdO9WpPKN",
				Title: "new title",
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, "object already exists").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name: "Key update and description for user with user_id",
			RequestBody: pb_key.UpdatePublicKey_Request{
				KeyId: "2511bbbe-c8d2-4e79-9e89-7e8f2b366a57",
				Key:   newKey(),
				Title: "new title",
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "user key data updated").
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "The public key has a broken structure",
			RequestBody: pb_key.UpdatePublicKey_Request{
				KeyId: "6437da7e-c9a4-463d-8b4d-2a7d1f730875",
				Key:   "ssh-ed25519 NzaC1lZDI1NTE5Wl2aY8FicEWNAlrQSuhU8SsXJErdO9WpPKN",
				Title: "new title",
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, "the public key has a broken structure").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
	}

	testCases["ROLE_USER"] = []tests.TestCase{
		{
			Name: "Update key",
			RequestBody: pb_key.UpdatePublicKey_Request{
				KeyId: "abe03b06-834b-4e35-a3f6-7e86df525c81",
				Key:   newKey(),
				Title: "new title",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "user key data updated").
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "Updated key and description, ignoring someone else's user_id",
			RequestBody: pb_key.UpdatePublicKey_Request{
				UserId: "0b792872-174a-4da4-9efa-f3fac872314e",
				KeyId:  "abe03b06-834b-4e35-a3f6-7e86df525c81",
				Key:    newKey(),
				Title:  "new title",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "user key data updated").
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "Unsuccessful renewal of a key that did not belong to an authorized user",
			RequestBody: pb_key.UpdatePublicKey_Request{
				KeyId: "2511bbbe-c8d2-4e79-9e89-7e8f2b366a57",
				Key:   newKey(),
				Title: "new title",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgNotFound).
				End(),
			RespondStatus: http.StatusNotFound,
		},
		{
			Name: "The public key has a broken structure",
			RequestBody: pb_key.UpdatePublicKey_Request{
				KeyId: "abe03b06-834b-4e35-a3f6-7e86df525c81",
				Key:   "ssh-ed25519 NzaC1lZDI1NTE5Wl2aY8FicEWNAlrQSuhU8SsXJErdO9WpPKN",
				Title: "new title",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, "the public key has a broken structure").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name: "The wrong parameter Key_id in the request",
			RequestBody: pb_key.UpdatePublicKey_Request{
				KeyId:  "00000000-0000",
				UserId: "00000000-0000",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateBody).
				Equal(`$.result.key`, "key is a required field").
				Equal(`$.result.keyid`, "keyId must be a valid UUID").
				Equal(`$.result.title`, "title is a required field").
				Equal(`$.result.userid`, "userId must be a valid UUID").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
	}

	for role, rtc := range testCases {
		t.Run(role, func(t *testing.T) {
			for _, tc := range rtc {
				t.Run(tc.Name, func(t *testing.T) {
					apiTest().
						Patch("/v1/keys").
						JSON(tc.RequestBody).
						Header("Authorization", "Bearer "+tc.RequestUser.Tokens.Access).
						Expect(t).
						Assert(tc.RespondBody).
						Status(tc.RespondStatus).
						End()
				})
			}
		})
	}
}

func TestHandler_deleteKey(t *testing.T) {
	t.Parallel()
	testCases := map[string][]tests.TestCase{}

	testCases["ROLE_USER_UNSPECIFIED"] = []tests.TestCase{
		{
			Name:         "Without parameters",
			RequestParam: map[string]string{},
			RequestUser:  &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},
	}

	testCases["ROLE_ADMIN"] = []tests.TestCase{
		{
			Name: "Deletion of your key",
			RequestParam: map[string]string{
				"user_id": adminInfo.UserID,
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "user key removed").
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "Deleting another user's key",
			RequestParam: map[string]string{
				"user_id": userInfo.UserID,
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "user key removed").
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "The wrong parameter key_id in the request",
			RequestParam: map[string]string{
				"key_id":  "00000000-0000",
				"user_id": "00000000-0000",
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateParams).
				End(),
			RespondStatus: http.StatusBadRequest,
		},
	}

	testCases["ROLE_USER"] = []tests.TestCase{
		{
			Name: "Deletion of your key",
			RequestParam: map[string]string{
				"user_id": userInfo.UserID,
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "user key removed").
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "Attempt to delete someone else's key",
			RequestParam: map[string]string{
				"key_id": "2511bbbe-c8d2-4e79-9e89-7e8f2b366a57",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgNotFound).
				End(),
			RespondStatus: http.StatusNotFound,
		},
		{
			Name: "Invalid key_id parameter",
			RequestParam: map[string]string{
				"key_id": "00000000-0000",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateParams).
				End(),
			RespondStatus: http.StatusBadRequest,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewKeyHandlersClient(testHandler.GRPC.Client)

	for role, rtc := range testCases {
		t.Run(role, func(t *testing.T) {
			for _, tc := range rtc {
				reqBody := tc.RequestParam.(map[string]string)

				if reqBody["user_id"] != "" {
					publicKey, _ := rClient.AddPublicKey(ctx, &pb.AddPublicKey_Request{
						UserId: reqBody["user_id"],
						Title:  "new test title",
						Key:    newKey(),
					})
					reqBody["key_id"] = publicKey.GetKeyId()
				}

				t.Run(tc.Name, func(t *testing.T) {
					apiTest().
						Delete("/v1/keys").
						QueryParams(reqBody).
						Header("Authorization", "Bearer "+tc.RequestUser.Tokens.Access).
						Expect(t).
						Assert(tc.RespondBody).
						Status(tc.RespondStatus).
						End()
				})
			}
		})
	}
}

func TestHandler_getGenerateNewKey(t *testing.T) {
	t.Parallel()
	testCases := map[string][]tests.TestCase{}

	testCases["ROLE_USER_UNSPECIFIED"] = []tests.TestCase{
		{
			Name:         "Without parameters",
			RequestParam: map[string]string{},
			RequestUser:  &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},
	}

	testCases["ROLE_ADMIN"] = []tests.TestCase{
		{
			Name:         "Without parameters",
			RequestParam: map[string]string{},
			RequestUser:  adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "ssh key pair created").
				Equal(`$.result.key_type`, "KEY_TYPE_ED25519").
				End(),
			RespondStatus: http.StatusOK,
		},
	}

	testCases["ROLE_USER"] = []tests.TestCase{
		{
			Name:         "Without parameters",
			RequestParam: map[string]string{},
			RequestUser:  userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "ssh key pair created").
				Equal(`$.result.key_type`, "KEY_TYPE_ED25519").
				End(),
			RespondStatus: http.StatusOK,
		},
	}

	for role, rtc := range testCases {
		t.Run(role, func(t *testing.T) {
			for _, tc := range rtc {
				t.Run(tc.Name, func(t *testing.T) {
					apiTest().
						Get("/v1/keys/generate").
						JSON(tc.RequestBody).
						Header("Authorization", "Bearer "+tc.RequestUser.Tokens.Access).
						Expect(t).
						Assert(tc.RespondBody).
						Status(tc.RespondStatus).
						End()
				})
			}
		})
	}
}
