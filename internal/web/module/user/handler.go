package user

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/mail"
	"github.com/werbot/werbot/internal/utils/validate"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/internal/grpc/proto/user"
)

// @Summary      Show information about user or list of all users
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        user_id     path     uuid false "User ID. Parameter Accessible with ROLE_ADMIN rights"
// @Success      200         {object} httputil.HTTPResponse{data=pb.ListUsersResponse}
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/users [get]
func (h *Handler) getUser(c *fiber.Ctx) error {
	input := new(pb.GetUser_Request)
	c.QueryParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	userID := userParameter.GetUserID(input.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUserHandlersClient(h.grpc.Client)

	// show all users
	if userParameter.IsUserAdmin() && input.GetUserId() == "" {
		pagination := httputil.GetPaginationFromCtx(c)
		users, err := rClient.ListUsers(ctx, &pb.ListUsers_Request{
			Limit:  pagination.GetLimit(),
			Offset: pagination.GetOffset(),
			SortBy: "id:ASC",
		})
		if err != nil {
			return httputil.ReturnGRPCError(c, err)
		}
		return httputil.StatusOK(c, "Users", users)
	}

	// show information about the key
	user, err := rClient.GetUser(ctx, &pb.GetUser_Request{
		UserId: userID,
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}

	if user == nil {
		return httputil.StatusNotFound(c, internal.ErrNotFound, nil)
	}

	// If RoleUser_ADMIN - show detailed information
	if userParameter.IsUserAdmin() {
		return httputil.StatusOK(c, "User information", user)
	}

	return httputil.StatusOK(c, "User information", &pb.GetUser_Response{
		Fio:   user.GetFio(),
		Name:  user.GetName(),
		Email: user.GetEmail(),
	})
}

// @Summary      Adding a new user (Only an administrator can added a new user).
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        req         body     pb.CreateUser_Request{}
// @Success      200         {object} httputil.HTTPResponse{data=pb.CreateUser_Response}
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/users [post]
func (h *Handler) addUser(c *fiber.Ctx) error {
	input := new(pb.CreateUser_Request)
	c.BodyParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	if !userParameter.IsUserAdmin() {
		return httputil.StatusNotFound(c, internal.ErrNotFound, nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUserHandlersClient(h.grpc.Client)

	user, err := rClient.CreateUser(ctx, &pb.CreateUser_Request{
		Fio:       input.GetFio(),
		Name:      input.GetName(),
		Email:     input.GetEmail(),
		Enabled:   input.GetEnabled(),
		Confirmed: input.GetConfirmed(),
		Password:  input.GetPassword(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "User added successfully", user)
}

// @Summary      Updating user information.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        user_id     path     int true "user_id"
// @Param        req         body     pb.UpdateUser_Request{}
// @Success      200         {object} httputil.HTTPResponse(data=pb.UpdateUser_Response)
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/users [patch]
func (h *Handler) patchUser(c *fiber.Ctx) error {
	input := new(pb.UpdateUser_Request)
	c.BodyParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	userID := userParameter.GetUserID(input.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUserHandlersClient(h.grpc.Client)

	// If RoleUser_ADMIN
	if userParameter.IsUserAdmin() {
		_, err := rClient.UpdateUser(ctx, &pb.UpdateUser_Request{
			UserId:    userID,
			Fio:       input.GetFio(),
			Name:      input.GetName(),
			Email:     input.GetEmail(),
			Enabled:   input.GetEnabled(),
			Confirmed: input.GetConfirmed(),
		})
		if err != nil {
			return httputil.ReturnGRPCError(c, err)
		}
		return httputil.StatusOK(c, "User data updated", nil)
	}

	_, err := rClient.UpdateUser(ctx, &pb.UpdateUser_Request{
		UserId: userID,
		Fio:    input.GetFio(),
		Email:  input.GetEmail(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "User data updated", nil)
}

// @Summary      Deleting a user
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        user_id     path     uuid true "user_id"
// @Param        token       path     uuid true "token"
// @Param        req         body     pb.DeleteUser_Request{}
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/users [delete]
func (h *Handler) deleteUser(c *fiber.Ctx) error {
	input := new(pb.DeleteUser_Request)
	//c.BodyParser(input)

	if err := protojson.Unmarshal(c.Body(), input); err != nil {
		fmt.Print(err)
	}

	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	userID := userParameter.GetUserID(input.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUserHandlersClient(h.grpc.Client)

	// step 1 - send email and token
	if input.GetPassword() != "" {
		response, err := rClient.DeleteUser(ctx, &pb.DeleteUser_Request{
			UserId: userID,
			Request: &pb.DeleteUser_Request_Password{
				Password: input.GetPassword(),
			},
		})
		if err != nil {
			return httputil.ReturnGRPCError(c, err)
		}

		mailData := map[string]string{
			"Name": response.GetName(),
			"Link": fmt.Sprintf("%s/profile/delete/%s", internal.GetString("APP_DSN", "https://app.werbot.com"), response.GetToken()),
		}
		go mail.Send(response.GetEmail(), "Account deletion confirmation", "account-deletion-confirmation", mailData)
		return httputil.StatusOK(c, "An email with instructions to delete your profile has been sent to your email", nil)
	}

	// step 2 - check token and delete user
	if input.GetToken() != "" {
		response, err := rClient.DeleteUser(ctx, &pb.DeleteUser_Request{
			UserId: userID,
			Request: &pb.DeleteUser_Request_Token{
				Token: c.Params("delete_token"),
			},
		})
		if err != nil {
			return httputil.ReturnGRPCError(c, err)
		}

		// send delete token to email
		mailData := map[string]string{
			"Name": response.GetName(),
		}
		go mail.Send(response.GetEmail(), "Account deletion information", "account-deletion-info", mailData)
		return httputil.StatusOK(c, "Account deleted", nil)
	}

	return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, nil)
}

// @Summary      Password update for a user.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        req         body     pb.UpdatePassword_Request{}
// @Param        user_id     path     int true "user_id"
// @Success      200         {object} httputil.HTTPResponse(data=pb.UpdatePassword_Response)
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/users/password [patch]
func (h *Handler) patchPassword(c *fiber.Ctx) error {
	input := new(pb.UpdatePassword_Request)
	c.BodyParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	userID := userParameter.GetUserID(input.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUserHandlersClient(h.grpc.Client)

	msg, err := rClient.UpdatePassword(ctx, &pb.UpdatePassword_Request{
		UserId:      userID,
		OldPassword: input.GetOldPassword(),
		NewPassword: input.GetNewPassword(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}

	return httputil.StatusOK(c, "Password updated", msg)
}
