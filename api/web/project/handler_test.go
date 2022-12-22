package project

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"
	pb_project "github.com/werbot/werbot/api/proto/project"
	pb_user "github.com/werbot/werbot/api/proto/user"
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/tests"
	"github.com/werbot/werbot/internal/web/middleware"
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
		Debug().
		HandlerFunc(testHandler.Handler)
}

func TestHandler_getProject(t *testing.T) {
	t.Parallel()

	testCases := []tests.TestCase{
		// Cases with an anonymous user
		{
			Name:         "ANONYMOUS_USER_getProject_01", // User not authorized
			RequestParam: map[string]string{},
			RequestUser:  &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},
		// Cases with an authorized user
		{
			Name:         "ROLE_USER_getProject_01", // List of all projects of user
			RequestParam: map[string]string{},
			RequestUser:  userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "Projects").
				Equal(`$.result.total`, float64(1)).
				Equal(`$.result.projects[0].owner_id`, userInfo.UserID).
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "ROLE_USER_getProject_02", // Error 404 on invalid input
			RequestParam: map[string]string{
				"owner_id":   uuid.New().String(),
				"project_id": uuid.New().String(),
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgNotFound).
				End(),
			RespondStatus: http.StatusNotFound,
		},
		{
			Name: "ROLE_USER_getProject_03", // Owner_id parameters are not checked
			RequestParam: map[string]string{
				"owner_id": uuid.New().String(),
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "Projects").
				Equal(`$.result.total`, float64(1)).
				Equal(`$.result.projects[0].owner_id`, userInfo.UserID).
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "ROLE_USER_getProject_04", // Error 404 when trying to display a non-existent project
			RequestParam: map[string]string{
				"project_id": uuid.New().String(),
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgNotFound).
				End(),
			RespondStatus: http.StatusNotFound,
		},
		{
			Name: "ROLE_USER_getProject_05", // Error 400 when trying to pass invalid parameters
			RequestParam: map[string]string{
				"project_id": "123",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateBody).
				Equal(`$.result.projectid`, "ProjectId must be a valid UUID").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		// Cases with a user with ADMIN rights
		{
			Name:         "ROLE_ADMIN_getProject_01", // List of all projects of user
			RequestParam: map[string]string{},
			RequestUser:  adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "Projects").
				Equal(`$.result.total`, float64(2)).
				Equal(`$.result.projects[0].owner_id`, adminInfo.UserID).
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "ROLE_ADMIN_getProject_02", // Error 404 on invalid input
			RequestParam: map[string]string{
				"owner_id":   uuid.New().String(),
				"project_id": uuid.New().String(),
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgNotFound).
				End(),
			RespondStatus: http.StatusNotFound,
		},
		{
			Name: "ROLE_ADMIN_getProject_03", // Error 404 when invalid owner ID
			RequestParam: map[string]string{
				"owner_id": uuid.New().String(),
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgNotFound).
				End(),
			RespondStatus: http.StatusNotFound,
		},
		{
			Name: "ROLE_ADMIN_getProject_04", // Error 404 when project of test-user@werbot.net, but not use owner_id
			RequestParam: map[string]string{
				"project_id": "69fbe29e-c955-41ad-b0c4-3a474cf01ea9",
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgNotFound).
				End(),
			RespondStatus: http.StatusNotFound,
		},
		{
			Name: "ROLE_ADMIN_getProject_05", // Shows all projects of another user (test-user@werbot.net)
			RequestParam: map[string]string{
				"owner_id": userInfo.UserID,
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "Projects").
				Equal(`$.result.total`, float64(1)).
				Equal(`$.result.projects[0].owner_id`, userInfo.UserID).
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "ROLE_ADMIN_getProject_06", // Information about another user's project (test-user@werbot.net)
			RequestParam: map[string]string{
				"owner_id":   userInfo.UserID,
				"project_id": "69fbe29e-c955-41ad-b0c4-3a474cf01ea9",
			},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "Project information").
				Equal(`$.result.title`, "test_project3").
				Equal(`$.result.login`, "testproject3").
				End(),
			RespondStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			apiTest().
				Get("/v1/projects").
				QueryParams(tc.RequestParam.(map[string]string)).
				Header("Authorization", "Bearer "+tc.RequestUser.Tokens.Access).
				Expect(t).
				Assert(tc.RespondBody).
				Status(tc.RespondStatus).
				End()
		})
	}
}

func TestHandler_addProject(t *testing.T) {
	t.Parallel()

	testCases := []tests.TestCase{
		// Cases with an anonymous user
		{
			Name:        "ANONYMOUS_USER_addProject_01", // User not authorized
			RequestBody: pb_project.CreateProject_Request{},
			RequestUser: &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},
		// Cases with an authorized user
		{
			Name: "ROLE_USER_addProject_01", // Error 400 when trying to pass invalid parameters
			RequestBody: pb_project.CreateProject_Request{
				Title: "",
				Login: "",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateBody).
				Equal(`$.result.login`, "Login is a required field").
				Equal(`$.result.title`, "Title is a required field").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name: "ROLE_USER_addProject_02", // Error 400 when trying to pass invalid parameters
			RequestBody: pb_project.CreateProject_Request{
				Title: "user",
				Login: "user999",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateBody).
				Equal(`$.result.title`, "Title must be at least 5 characters in length").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name: "ROLE_USER_addProject_03", // Error 400 when trying to pass invalid parameters
			RequestBody: pb_project.CreateProject_Request{
				Login: "user999",
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
			Name: "ROLE_USER_addProject_04",
			RequestBody: pb_project.CreateProject_Request{
				Title: "user999",
				Login: "user999",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "Project added").
				End(),
			RespondStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			resp := apiTest().
				Post("/v1/projects").
				JSON(tc.RequestBody).
				Header("Authorization", "Bearer "+tc.RequestUser.Tokens.Access).
				Expect(t).
				Assert(tc.RespondBody).
				Status(tc.RespondStatus).
				End()

			// delete added project
			data := map[string]pb_project.CreateProject_Response{}
			json.NewDecoder(resp.Response.Body).Decode(&data)
			if data["result"].ProjectId != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				rClient := pb_project.NewProjectHandlersClient(testHandler.GRPC.Client)

				rClient.DeleteProject(ctx, &pb_project.DeleteProject_Request{
					OwnerId:   tc.RequestUser.UserID,
					ProjectId: data["result"].ProjectId,
				})
			}
		})
	}
}

func TestHandler_patchProject(t *testing.T) {
	t.Parallel()

	testCases := []tests.TestCase{
		// Cases with an anonymous user
		{
			Name:        "ANONYMOUS_USER_patchProject_01", // User not authorized
			RequestBody: pb_project.UpdateProject_Request{},
			RequestUser: &tests.UserInfo{},
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgUnauthorized).
				End(),
			RespondStatus: http.StatusUnauthorized,
		},
		// Cases with an authorized user
		{
			Name:        "ROLE_USER_patchProject_01", // Error 400 when trying to pass invalid parameters
			RequestBody: pb_project.UpdateProject_Request{},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				Equal(`$.message`, internal.MsgFailedToValidateBody).
				Equal(`$.result.projectid`, "ProjectId is a required field").
				Equal(`$.result.title`, "Title is a required field").
				End(),
			RespondStatus: http.StatusBadRequest,
		},
		{
			Name: "ROLE_USER_patchProject_02", // Error 400 when trying to pass invalid parameters
			RequestBody: pb_project.UpdateProject_Request{
				ProjectId: "d958ee44-a960-420e-9bbf-c7a35084c4aa",
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
			Name: "ROLE_USER_patchProject_03", // Fake status 200 (real 404), non-existent project ip
			RequestBody: pb_project.UpdateProject_Request{
				ProjectId: "00000000-0000-0000-0000-000000000000",
				Title:     "user999",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "Project data updated").
				End(),
			RespondStatus: http.StatusOK,
		},
		{
			Name: "ROLE_USER_patchProject_04", // Successful data update
			RequestBody: pb_project.UpdateProject_Request{
				ProjectId: "d958ee44-a960-420e-9bbf-c7a35084c4aa",
				Title:     "user999",
			},
			RequestUser: userInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, true).
				Equal(`$.message`, "Project data updated").
				End(),
			RespondStatus: http.StatusOK,
		},
		// Cases with a user with ADMIN rights
		{
			Name:        "ROLE_ADMIN_patchProject_01",
			RequestBody: pb_project.UpdateProject_Request{},
			RequestUser: adminInfo,
			RespondBody: jsonpath.Chain().
				Equal(`$.success`, false).
				End(),
			RespondStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			apiTest().
				Patch("/v1/projects").
				JSON(tc.RequestBody).
				Header("Authorization", "Bearer "+tc.RequestUser.Tokens.Access).
				Expect(t).
				Assert(tc.RespondBody).
				Status(tc.RespondStatus).
				End()
		})
	}
}
