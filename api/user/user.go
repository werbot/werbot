package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/werbot/werbot/internal"
	userpb "github.com/werbot/werbot/internal/grpc/user/proto"
	"github.com/werbot/werbot/internal/mail"
	"github.com/werbot/werbot/internal/trace"
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
		return webutil.FromGRPC(c, errors.New("incorrect parameters"))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(userpb.User_RequestMultiError) {
			e := err.(userpb.User_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
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
			return webutil.FromGRPC(c, err)
		}
		if users.GetTotal() == 0 {
			return webutil.FromGRPC(c, status.Error(codes.NotFound, "not found"))
		}

		return webutil.StatusOK(c, "users", users)
	}

	// show information about the key
	user, err := rClient.User(ctx, &userpb.User_Request{
		UserId: userID,
	})
	if err != nil {
		return webutil.FromGRPC(c, err)
	}
	// if user == nil {
	//	return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	//}

	// If Role_admin - show detailed information
	if userParameter.IsUserAdmin() {
		return webutil.StatusOK(c, "user information", user)
	}

	return webutil.StatusOK(c, "user information", &userpb.User_Response{
		Login:   user.GetLogin(),
		Name:    user.GetName(),
		Surname: user.GetSurname(),
		Email:   user.GetEmail(),
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
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(userpb.AddUser_RequestMultiError) {
			e := err.(userpb.AddUser_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	if !userParameter.IsUserAdmin() {
		return webutil.FromGRPC(c, status.Error(codes.NotFound, "not found"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := userpb.NewUserHandlersClient(h.Grpc.Client)
	user, err := rClient.AddUser(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "user added", user)
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
func (h *Handler) updateUser(c *fiber.Ctx) error {
	request := new(userpb.UpdateUser_Request)

	if err := protojson.Unmarshal(c.Body(), request); err != nil {
		h.log.Error(err).Send()
		return webutil.FromGRPC(c, errors.New("incorrect parameters"))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(userpb.UpdateUser_RequestMultiError) {
			e := err.(userpb.UpdateUser_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := userpb.NewUserHandlersClient(h.Grpc.Client)

	var err error
	switch request.Request.(type) {
	case *userpb.UpdateUser_Request_Info:
		setting := new(userpb.UpdateUser_Request_Info)
		setting.Info = new(userpb.UpdateUser_Info)
		setting.Info.Email = request.GetInfo().GetEmail()
		setting.Info.Name = request.GetInfo().GetName()
		setting.Info.Surname = request.GetInfo().GetSurname()
		request.Request = setting

	case *userpb.UpdateUser_Request_Enabled:
		setting := new(userpb.UpdateUser_Request_Enabled)
		setting.Enabled = request.GetEnabled()
		request.Request = setting

	case *userpb.UpdateUser_Request_Confirmed:
		setting := new(userpb.UpdateUser_Request_Confirmed)
		setting.Confirmed = request.GetConfirmed()
		request.Request = setting

	default:
		return webutil.FromGRPC(c, errors.New("bad rule")) // msgBadRule
	}

	/*
		// If Role_admin
		if userParameter.IsUserAdmin() {
			_, err := rClient.UpdateUser(ctx, &userpb.UpdateUser_Request{
				UserId: request.UserId,
				Setting: &userpb.UpdateUser_Request_Info{
					Info: &userpb.UpdateUser_Info{
						Login:     request.GetInfo().GetLogin(),
						Email:     request.GetInfo().GetEmail(),
						Name:      request.GetInfo().GetName(),
						Surname:   request.GetInfo().GetSurname(),
						Enabled:   request.GetInfo().GetEnabled(),
						Confirmed: request.GetInfo().GetConfirmed(),
					},
				},
			})
			if err != nil {
				return webutil.FromGRPC(c, err)
			}

			return webutil.StatusOK(c, msgUserUpdated, nil)
		}
	*/

	_, err = rClient.UpdateUser(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "user updated", nil)
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
		return webutil.FromGRPC(c, err, multiError)
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
			return webutil.FromGRPC(c, err)
		}

		mailData := map[string]string{
			"Login": response.GetLogin(),
			"Link":  fmt.Sprintf("%s/profile/delete/%s", internal.GetString("APP_DSN", "http://localhost:5173"), response.GetToken()),
		}
		go mail.Send(response.GetEmail(), "user deletion confirmation", "account-deletion-confirmation", mailData)
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
			return webutil.FromGRPC(c, err)
		}

		// send delete token to email
		mailData := map[string]string{
			"Login": response.GetLogin(),
		}
		go mail.Send(response.GetEmail(), "user deleted", "account-deletion-info", mailData)
		return webutil.StatusOK(c, "user deleted", nil)
	}

	return webutil.FromGRPC(c, errors.New("incorrect parameters"))
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
func (h *Handler) updatePassword(c *fiber.Ctx) error {
	request := new(userpb.UpdatePassword_Request)

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(userpb.UpdatePassword_RequestMultiError) {
			e := err.(userpb.UpdatePassword_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := userpb.NewUserHandlersClient(h.Grpc.Client)
	msg, err := rClient.UpdatePassword(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "password updated", msg)
}
