package user

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/mail"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"

	pb "github.com/werbot/werbot/api/proto/user"
)

// @Summary      Show information about user or list of all users
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        user_id     path     uuid false "User ID. Parameter Accessible with ROLE_ADMIN rights"
// @Success      200         {object} webutil.HTTPResponse{data=pb.ListUsersResponse}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/users [get]
func (h *handler) getUser(c *fiber.Ctx) error {
	request := new(pb.User_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.User_RequestMultiError) {
			e := err.(pb.User_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUserHandlersClient(h.Grpc.Client)

	// show all users
	if userParameter.IsUserAdmin() && request.GetUserId() == "" {
		pagination := webutil.GetPaginationFromCtx(c)
		users, err := rClient.ListUsers(ctx, &pb.ListUsers_Request{
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
	user, err := rClient.User(ctx, &pb.User_Request{
		UserId: userID,
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}
	// if user == nil {
	//	return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	//}

	// If RoleUser_ADMIN - show detailed information
	if userParameter.IsUserAdmin() {
		return webutil.StatusOK(c, msgUserInfo, user)
	}

	return webutil.StatusOK(c, msgUserInfo, &pb.User_Response{
		Fio:   user.GetFio(),
		Name:  user.GetName(),
		Email: user.GetEmail(),
	})
}

// @Summary      Adding a new user (Only an administrator can added a new user).
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        req         body     pb.AddUser_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=pb.AddUser_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/users [post]
func (h *handler) addUser(c *fiber.Ctx) error {
	request := new(pb.AddUser_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.AddUser_RequestMultiError) {
			e := err.(pb.AddUser_RequestValidationError)
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
	rClient := pb.NewUserHandlersClient(h.Grpc.Client)

	user, err := rClient.AddUser(ctx, &pb.AddUser_Request{
		Fio:       request.GetFio(),
		Name:      request.GetName(),
		Email:     request.GetEmail(),
		Enabled:   request.GetEnabled(),
		Confirmed: request.GetConfirmed(),
		Password:  request.GetPassword(),
	})
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
// @Param        req         body     pb.UpdateUser_Request{}
// @Success      200         {object} webutil.HTTPResponse(data=pb.UpdateUser_Response)
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/users [patch]
func (h *handler) patchUser(c *fiber.Ctx) error {
	request := new(pb.UpdateUser_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.UpdateUser_RequestMultiError) {
			e := err.(pb.UpdateUser_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUserHandlersClient(h.Grpc.Client)

	// If RoleUser_ADMIN
	if userParameter.IsUserAdmin() {
		_, err := rClient.UpdateUser(ctx, &pb.UpdateUser_Request{
			UserId:    userID,
			Fio:       request.GetFio(),
			Name:      request.GetName(),
			Email:     request.GetEmail(),
			Enabled:   request.GetEnabled(),
			Confirmed: request.GetConfirmed(),
		})
		if err != nil {
			return webutil.FromGRPC(c, h.log, err)
		}

		return webutil.StatusOK(c, msgUserUpdated, nil)
	}

	_, err := rClient.UpdateUser(ctx, &pb.UpdateUser_Request{
		UserId: userID,
		Fio:    request.GetFio(),
		Email:  request.GetEmail(),
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
// @Param        req         body     pb.DeleteUser_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/users [delete]
func (h *handler) deleteUser(c *fiber.Ctx) error {
	request := new(pb.DeleteUser_Request)
	// c.BodyParser(request)

	if err := protojson.Unmarshal(c.Body(), request); err != nil {
		fmt.Print(err)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.DeleteUser_RequestMultiError) {
			e := err.(pb.DeleteUser_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUserHandlersClient(h.Grpc.Client)

	// step 1 - send email and token
	if request.GetPassword() != "" {
		response, err := rClient.DeleteUser(ctx, &pb.DeleteUser_Request{
			UserId: userID,
			Request: &pb.DeleteUser_Request_Password{
				Password: request.GetPassword(),
			},
		})
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
		response, err := rClient.DeleteUser(ctx, &pb.DeleteUser_Request{
			UserId: userID,
			Request: &pb.DeleteUser_Request_Token{
				Token: c.Params("delete_token"),
			},
		})
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
// @Param        req         body     pb.UpdatePassword_Request{}
// @Param        user_id     path     int true "user_id"
// @Success      200         {object} webutil.HTTPResponse(data=pb.UpdatePassword_Response)
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/users/password [patch]
func (h *handler) patchPassword(c *fiber.Ctx) error {
	request := new(pb.UpdatePassword_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.UpdatePassword_RequestMultiError) {
			e := err.(pb.UpdatePassword_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUserHandlersClient(h.Grpc.Client)

	msg, err := rClient.UpdatePassword(ctx, &pb.UpdatePassword_Request{
		UserId:      userID,
		OldPassword: request.GetOldPassword(),
		NewPassword: request.GetNewPassword(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgPasswordUpdated, msg)
}
