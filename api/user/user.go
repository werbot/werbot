package user

import (
	"github.com/gofiber/fiber/v2"

	userpb "github.com/werbot/werbot/internal/core/user/proto/user"
	"github.com/werbot/werbot/internal/event"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Retrieve users
// @Description Retrieves a list of users with pagination and sorting options. Access restricted to admin users.
// @Tags users
// @Accept json
// @Produce json
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Param sort_by query string false "Sort by for pagination"
// @Success 200 {object} webutil.HTTPResponse{result=userpb.Users_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/users/list [get]
func (h *Handler) users(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)

	// access only for admin
	if !sessionData.IsUserAdmin() {
		return webutil.StatusNotFound(c, nil)
	}

	pagination := webutil.GetPaginationFromCtx(c)
	request := &userpb.Users_Request{
		IsAdmin: sessionData.IsUserAdmin(),
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		SortBy:  `"user"."created_at":ASC`,
	}

	rClient := userpb.NewUserHandlersClient(h.Grpc)
	users, err := rClient.Users(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(users)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Users", result)
}

// @Summary Retrieve user
// @Description Retrieves details of a specific user by user ID. Access level depends on whether the requester is an admin.
// @Tags users
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Success 200 {object} webutil.HTTPResponse{result=userpb.User_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/users [get]
func (h *Handler) user(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &userpb.User_Request{
		IsAdmin: sessionData.IsUserAdmin(),
		UserId:  sessionData.UserID(c.Query("user_id")),
	}

	rClient := userpb.NewUserHandlersClient(h.Grpc)
	user, err := rClient.User(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(user)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "User", result)
}

// @Summary Add a new user
// @Description Adds a new user to the system. Only accessible by admin users.
// @Tags users
// @Accept json
// @Produce json
// @Param user body userpb.AddUser_Request true "Add User Request"
// @Success 200 {object} webutil.HTTPResponse{result=userpb.AddUser_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/users [post]
func (h *Handler) addUser(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)

	// access only for admin
	if !sessionData.IsUserAdmin() {
		return webutil.StatusNotFound(c, nil)
	}

	request := &userpb.AddUser_Request{
		IsAdmin: sessionData.IsUserAdmin(),
	}

	_ = webutil.Parse(c, request).Body(false)

	rClient := userpb.NewUserHandlersClient(h.Grpc)
	user, err := rClient.AddUser(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(user)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Profile(sessionData.User.GetUserId(), event.ProfileProfile, event.OnCreate, request)

	return webutil.StatusOK(c, "User added", result)
}

// @Summary Update user information
// @Description Updates the user's details based on the provided request data.
// @Tags users
// @Accept json
// @Produce json
// @Param user_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param body body userpb.UpdateUser_Request true
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/users [put]
func (h *Handler) updateUser(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &userpb.UpdateUser_Request{
		IsAdmin: sessionData.IsUserAdmin(),
		UserId:  sessionData.UserID(c.Query("user_id")),
	}

	_ = webutil.Parse(c, request).Body(true)

	rClient := userpb.NewUserHandlersClient(h.Grpc)
	if _, err := rClient.UpdateUser(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	var eventType event.EventType
	switch request.GetSetting().(type) {
	case *userpb.UpdateUser_Request_Alias, *userpb.UpdateUser_Request_Email, *userpb.UpdateUser_Request_Name, *userpb.UpdateUser_Request_Surname:
		eventType = event.OnUpdate
	case *userpb.UpdateUser_Request_Active, *userpb.UpdateUser_Request_Confirmed:
		if request.GetActive() || request.GetConfirmed() {
			eventType = event.OnOffline
		} else {
			eventType = event.OnOnline
		}
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Profile(request.GetUserId(), event.ProfileProfile, eventType, request)

	return webutil.StatusOK(c, "User updated", nil)
}

// @Summary Delete User
// @Description Deletes a user either by sending an email with a token (step 1) or by verifying the token and deleting the user (step 2).
// @Tags user2
// @Accept json
// @Produce json
// @Param user_id query string true "User ID"
// @Param token path string false "Token"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/users/delete [post, delete]
func (h *Handler) deleteUser(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &userpb.DeleteUser_Request{
		UserId: sessionData.UserID(c.Query("user_id")),
	}

	// using on step 1
	if c.Method() == "POST" {
		_ = webutil.Parse(c, request).Body(true)
	}

	// using on step 2
	if c.Method() == "DELETE" {
		request.Request = &userpb.DeleteUser_Request_Token{
			Token: c.Params("token"),
		}
	}

	var message, description string
	var eventType event.EventType
	var metaData map[string]any

	switch request.GetRequest().(type) {
	case *userpb.DeleteUser_Request_Password: // step 1 - send email and token
		message = "Request for delete"
		description = "An email with instructions to delete your profile has been sent to your email"
		eventType = event.OnMessage
		metaData = event.Metadata{
			"subject": "user deletion confirmation",
		}
	case *userpb.DeleteUser_Request_Token: // step 2 - check token and delete user
		message = "User deleted"
		eventType = event.OnInactive
		metaData = event.Metadata{
			"subject": "user deleted",
		}
	default:
		return webutil.StatusBadRequest(c, map[string]string{
			"request": "exactly one field is required in oneof",
		})
	}

	rClient := userpb.NewUserHandlersClient(h.Grpc)
	if _, err := rClient.DeleteUser(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	go event.New(h.Grpc).Web(c, sessionData).Profile(request.GetUserId(), event.ProfileProfile, eventType, metaData)

	return webutil.StatusOK(c, message, description)
}

// @Summary Update user password
// @Description Updates the password for a given user ID.
// @Tags users
// @Accept json
// @Produce json
// @Param body body userpb.UpdatePassword_Request true "Update Password Request"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/users/password [patch]
func (h *Handler) updatePassword(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &userpb.UpdatePassword_Request{
		UserId: sessionData.User.GetUserId(),
	}

	_ = webutil.Parse(c, request).Body()

	rClient := userpb.NewUserHandlersClient(h.Grpc)
	msg, err := rClient.UpdatePassword(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	ghoster.Secrets(msg, false)
	go event.New(h.Grpc).Web(c, sessionData).Profile(request.GetUserId(), event.ProfileProfile, event.OnUpdate, msg)

	return webutil.StatusOK(c, "Password updated", msg)
}
