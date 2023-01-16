package user

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/protobuf/encoding/protojson"

	userpb "github.com/werbot/werbot/api/proto/user"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/mail"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"
)

// @Summary      Show information about user or list of all users
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        user_id     path     uuid false "User ID. Parameter Accessible with ROLE_ADMIN rights"
// @Success      200         {object} webutil.HTTPResponse{data=userpb.ListUsersResponse}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/users [get]
func (h *Handler) getUser(c *fiber.Ctx) error {
	request := new(userpb.User_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(userpb.User_RequestMultiError) {
			e := err.(userpb.User_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := userpb.NewUserHandlersClient(h.Grpc.Client)

	// show all users
	if userParameter.IsUserAdmin() && request.GetUserId() == "" {
		pagination := webutil.GetPaginationFromCtx(c)
		users, err := rClient.ListUsers(ctx, &userpb.ListUsers_Request{
			Limit:  pagination.GetLimit(),
			Offset: pagination.GetOffset(),
			SortBy: "id:ASC",
		})
		if err != nil {
			return webutil.FromGRPC(c, h.log, err)
		}
		if users.GetTotal() == 0 {
			return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
		}

		return webutil.StatusOK(c, msgUsers, users)
	}

	// show information about the key
	user, err := rClient.User(ctx, &userpb.User_Request{
		UserId: userID,
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}
	// if user == nil {
	//	return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	//}

	// If Role_admin - show detailed information
	if userParameter.IsUserAdmin() {
		return webutil.StatusOK(c, msgUserInfo, user)
	}

	return webutil.StatusOK(c, msgUserInfo, &userpb.User_Response{
		Fio:   user.GetFio(),
		Name:  user.GetName(),
		Email: user.GetEmail(),
	})
}

// @Summary      Adding a new user (Only an administrator can added a new user).
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        req         body     userpb.AddUser_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=userpb.AddUser_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/users [post]
func (h *Handler) addUser(c *fiber.Ctx) error {
	request := new(userpb.AddUser_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(userpb.AddUser_RequestMultiError) {
			e := err.(userpb.AddUser_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	if !userParameter.IsUserAdmin() {
		return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := userpb.NewUserHandlersClient(h.Grpc.Client)
	user, err := rClient.AddUser(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgUserAdded, user)
}

// @Summary      Updating user information.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        user_id     path     int true "user_id"
// @Param        req         body     userpb.UpdateUser_Request{}
// @Success      200         {object} webutil.HTTPResponse(data=userpb.UpdateUser_Response)
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/users [patch]
func (h *Handler) patchUser(c *fiber.Ctx) error {
	request := new(userpb.UpdateUser_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(userpb.UpdateUser_RequestMultiError) {
			e := err.(userpb.UpdateUser_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := userpb.NewUserHandlersClient(h.Grpc.Client)

	// If Role_admin
	if userParameter.IsUserAdmin() {
		_, err := rClient.UpdateUser(ctx, &userpb.UpdateUser_Request{
			UserId: request.UserId,
			Setting: &userpb.UpdateUser_Request_Info{
				Info: &userpb.UpdateUser_Info{
					Name:      request.GetInfo().GetName(),
					Email:     request.GetInfo().GetEmail(),
					Fio:       request.GetInfo().GetFio(),
					Enabled:   request.GetInfo().GetEnabled(),
					Confirmed: request.GetInfo().GetConfirmed(),
				},
			},
		})
		if err != nil {
			return webutil.FromGRPC(c, h.log, err)
		}

		return webutil.StatusOK(c, msgUserUpdated, nil)
	}

	_, err := rClient.UpdateUser(ctx, &userpb.UpdateUser_Request{
		UserId: request.UserId,
		Setting: &userpb.UpdateUser_Request_Info{
			Info: &userpb.UpdateUser_Info{
				Email: request.GetInfo().GetEmail(),
				Fio:   request.GetInfo().GetFio(),
			},
		},
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgUserUpdated, nil)
}

// @Summary      Deleting a user
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        user_id     path     uuid true "user_id"
// @Param        token       path     uuid true "token"
// @Param        req         body     userpb.DeleteUser_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/users [delete]
func (h *Handler) deleteUser(c *fiber.Ctx) error {
	request := new(userpb.DeleteUser_Request)
	// c.BodyParser(request)

	if err := protojson.Unmarshal(c.Body(), request); err != nil {
		fmt.Print(err)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(userpb.DeleteUser_RequestMultiError) {
			e := err.(userpb.DeleteUser_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := userpb.NewUserHandlersClient(h.Grpc.Client)

	// step 1 - send email and token
	if request.GetPassword() != "" {
		response, err := rClient.DeleteUser(ctx, request)
		/*
			response, err := rClient.DeleteUser(ctx, &userpb.DeleteUser_Request{
				UserId: userID,
				Request: &userpb.DeleteUser_Request_Password{
					Password: request.GetPassword(),
				},
			})
		*/
		if err != nil {
			return webutil.FromGRPC(c, h.log, err)
		}

		mailData := map[string]string{
			"Name": response.GetName(),
			"Link": fmt.Sprintf("%s/profile/delete/%s", internal.GetString("APP_DSN", "https://app.werbot.com"), response.GetToken()),
		}
		go mail.Send(response.GetEmail(), msgUserDeletionConfirmation, "account-deletion-confirmation", mailData)
		return webutil.StatusOK(c, "an email with instructions to delete your profile has been sent to your email", nil)
	}

	// step 2 - check token and delete user
	if request.GetToken() != "" {
		token := new(userpb.DeleteUser_Request_Token)
		token.Token = c.Params("delete_token")
		response, err := rClient.DeleteUser(ctx, request)
		/*
			response, err := rClient.DeleteUser(ctx, &userpb.DeleteUser_Request{
				UserId: userID,
				Request: &userpb.DeleteUser_Request_Token{
					Token: c.Params("delete_token"),
				},
			})
		*/
		if err != nil {
			return webutil.FromGRPC(c, h.log, err)
		}

		// send delete token to email
		mailData := map[string]string{
			"Name": response.GetName(),
		}
		go mail.Send(response.GetEmail(), msgUserDeleted, "account-deletion-info", mailData)
		return webutil.StatusOK(c, msgUserDeleted, nil)
	}

	return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
}

// @Summary      Password update for a user.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        req         body     userpb.UpdatePassword_Request{}
// @Param        user_id     path     int true "user_id"
// @Success      200         {object} webutil.HTTPResponse(data=userpb.UpdatePassword_Response)
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/users/password [patch]
func (h *Handler) patchPassword(c *fiber.Ctx) error {
	request := new(userpb.UpdatePassword_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(userpb.UpdatePassword_RequestMultiError) {
			e := err.(userpb.UpdatePassword_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := userpb.NewUserHandlersClient(h.Grpc.Client)
	msg, err := rClient.UpdatePassword(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgPasswordUpdated, msg)
}
