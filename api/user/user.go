package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/grpc"
	userpb "github.com/werbot/werbot/internal/grpc/user/proto"
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
	request := &userpb.User_Request{}

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, nil)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := userpb.NewUserHandlersClient(h.Grpc)

	// show all users
	if userParameter.IsUserAdmin() && request.GetUserId() == "" {
		pagination := webutil.GetPaginationFromCtx(c)
		users, err := rClient.ListUsers(ctx, &userpb.ListUsers_Request{
			Limit:  pagination.Limit,
			Offset: pagination.Offset,
			SortBy: "id:ASC",
		})
		if err != nil {
			return webutil.FromGRPC(c, err)
		}
		if users.GetTotal() == 0 {
			return webutil.StatusNotFound(c, nil)
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
	request := &userpb.AddUser_Request{}

	if err := c.BodyParser(request); err != nil {
		return webutil.StatusBadRequest(c, "The body of the request is damaged")
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	userParameter := middleware.AuthUser(c)
	if !userParameter.IsUserAdmin() {
		return webutil.StatusNotFound(c, nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := userpb.NewUserHandlersClient(h.Grpc)
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
	request := &userpb.UpdateUser_Request{}

	if err := protojson.Unmarshal(c.Body(), request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, nil)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := userpb.NewUserHandlersClient(h.Grpc)

	switch request.Request.(type) {
	case *userpb.UpdateUser_Request_Info:
		setting := &userpb.UpdateUser_Request_Info{}
		setting.Info = &userpb.UpdateUser_Info{}
		setting.Info.Email = request.GetInfo().GetEmail()
		setting.Info.Name = request.GetInfo().GetName()
		setting.Info.Surname = request.GetInfo().GetSurname()
		request.Request = setting

	case *userpb.UpdateUser_Request_Enabled:
		setting := &userpb.UpdateUser_Request_Enabled{}
		setting.Enabled = request.GetEnabled()
		request.Request = setting

	case *userpb.UpdateUser_Request_Confirmed:
		setting := &userpb.UpdateUser_Request_Confirmed{}
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

	if _, err := rClient.UpdateUser(ctx, request); err != nil {
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
	request := &userpb.DeleteUser_Request{}
	// c.BodyParser(request)

	if err := protojson.Unmarshal(c.Body(), request); err != nil {
		fmt.Print(err)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := userpb.NewUserHandlersClient(h.Grpc)

	// step 1 - send email and token
	if request.GetPassword() != "" {
		response, err := rClient.DeleteUser(ctx, request)
		if err != nil {
			return webutil.FromGRPC(c, err)
		}

		/*
			response, err := rClient.DeleteUser(ctx, &userpb.DeleteUser_Request{
				UserId: userID,
				Request: &userpb.DeleteUser_Request_Password{
					Password: request.GetPassword(),
				},
			})
		*/

		mailData := map[string]string{
			"Login": response.GetLogin(),
			"Link":  fmt.Sprintf("%s/profile/delete/%s", internal.GetString("APP_DSN", "http://localhost:5173"), response.GetToken()),
		}
		go mail.Send(response.GetEmail(), "user deletion confirmation", "account-deletion-confirmation", mailData)
		return webutil.StatusOK(c, "an email with instructions to delete your profile has been sent to your email", nil)
	}

	// step 2 - check token and delete user
	if request.GetToken() != "" {
		token := &userpb.DeleteUser_Request_Token{}
		token.Token = c.Params("delete_token")
		response, err := rClient.DeleteUser(ctx, request)
		if err != nil {
			return webutil.FromGRPC(c, err)
		}

		/*
			response, err := rClient.DeleteUser(ctx, &userpb.DeleteUser_Request{
				UserId: userID,
				Request: &userpb.DeleteUser_Request_Token{
					Token: c.Params("delete_token"),
				},
			})
		*/

		// send delete token to email
		mailData := map[string]string{
			"Login": response.GetLogin(),
		}
		go mail.Send(response.GetEmail(), "user deleted", "account-deletion-info", mailData)
		return webutil.StatusOK(c, "user deleted", nil)
	}

	return webutil.StatusBadRequest(c, nil)
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
	request := &userpb.UpdatePassword_Request{}

	if err := c.BodyParser(request); err != nil {
		return webutil.StatusBadRequest(c, "The body of the request is damaged")
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := userpb.NewUserHandlersClient(h.Grpc)
	msg, err := rClient.UpdatePassword(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "password updated", msg)
}
