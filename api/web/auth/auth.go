package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/mail"
	"github.com/werbot/werbot/internal/utils/validate"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/api/proto/user"
)

// @Summary      Authorization in the system
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        email    body     pb.SignIn_Request true "Email"
// @Param        password body     pb.SignIn_Request true "Password"
// @Success      200      {object} jwt.Tokens
// @Failure      400,500  {object} httputil.HTTPResponse
// @Router       /auth/signin [post]
func (h *handler) signIn(c *fiber.Ctx) error {
	input := &pb.SignIn_Request{}
	c.BodyParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUserHandlersClient(h.Grpc.Client)

	user, err := rClient.SignIn(ctx, &pb.SignIn_Request{
		Email:    input.GetEmail(),
		Password: input.GetPassword(),
	})
	if err != nil {
		se, _ := status.FromError(err)
		if se.Message() != "" {
			return httputil.StatusBadRequest(c, se.Message(), nil)
		}
		return httputil.InternalServerError(c, internal.ErrUnexpectedError, nil)
	}

	sub := uuid.New().String()
	newToken, err := jwt.New(&pb.UserParameters{
		UserName: "TODO",
		UserId:   user.GetUserId(),
		Roles:    user.GetRole(),
		Sub:      sub,
	})
	if err != nil {
		h.log.Error(err).Msg("Failed to create token")
		return httputil.InternalServerError(c, "Failed to create token", nil)
	}

	// We write user_id (user.user.userid) in the database, since if Access_key will not know which user to create a new one
	if !jwt.AddToken(h.Cache, sub, user.GetUserId()) {
		return httputil.InternalServerError(c, "Failed to set cache", nil)
	}

	return httputil.StatusOK(c, "", jwt.Tokens{
		Access:  newToken.Tokens.Access,
		Refresh: newToken.Tokens.Refresh,
	})
}

// @Summary      Sign out from the system
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200 {object} httputil.HTTPResponse
// @Router       /auth/logout [post]
func (h *handler) logout(c *fiber.Ctx) error {
	userParameter := middleware.AuthUser(c)
	jwt.DeleteToken(h.Cache, userParameter.UserSub())
	return httputil.StatusOK(c, "Successfully logged out", nil)
}

// @Summary      Re-creation of new tokens
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        refresh_token body     string true "Refresh token"
// @Success      200           {object} jwt.Tokens
// @Failure      400,404,500   {object} httputil.HTTPResponse
// @Router       /auth/refresh [post]
func (h *handler) refresh(c *fiber.Ctx) error {
	input := new(jwt.Tokens)
	if err := c.BodyParser(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrBadQueryParams, nil)
	}

	claims, err := jwt.Parse(input.Refresh)
	if err != nil {
		return httputil.StatusBadRequest(c, err.Error(), nil)
	}

	sub := jwt.GetClaimSub(*claims)
	userID, err := h.Cache.Get(fmt.Sprintf("ref_token::%s", sub))

	if !jwt.ValidateToken(h.Cache, sub) {
		return httputil.StatusBadRequest(c, "Your token has been revoked", nil)
	}
	jwt.DeleteToken(h.Cache, sub)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUserHandlersClient(h.Grpc.Client)

	user, _ := rClient.GetUser(ctx, &pb.GetUser_Request{
		UserId: userID,
	})

	newToken, err := jwt.New(&pb.UserParameters{
		UserName: "UserName",
		UserId:   user.GetUserId(),
		Roles:    user.GetRole(),
		Sub:      sub,
	})
	if err != nil {
		return httputil.StatusBadRequest(c, "Failed to create token", nil)
	}

	jwt.AddToken(h.Cache, sub, userID)

	tokens := &jwt.Tokens{
		Access:  newToken.Tokens.Access,
		Refresh: newToken.Tokens.Refresh,
	}

	return httputil.StatusOK(c, "", tokens)
}

// @Summary      Password reset
// @Description  Reset password for existing user by email. Reset password occurs in 3 stages:
// @Description  1. Sending an email with a reset token
// @Description  2. Checking the token from the email (use reset_token)
// @Description  3. Saving a new password using a previously sent token (use reset_token)
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        email       path     string true "Step1: user email"
// @Param        reset_token path     uuid true "Step2: reset token"
// @Param        password    path     string true "Step2: new password"
// @Success      200         {object} httputil.HTTPResponse{data=user.ResetPassword_Response}
// @Failure      400,500     {object} httputil.HTTPResponse
// @Router       /auth/password_reset [post]
func (h *handler) resetPassword(c *fiber.Ctx) error {
	request := new(pb.ResetPassword_Request)
	if err := protojson.Unmarshal(c.Body(), request); err != nil {
		fmt.Print(err)
	}

	request.Token = c.Params("reset_token")
	if err := validate.Struct(request); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	// Sending an email with a verification link
	if request.GetEmail() != "" {
		request.Request = &pb.ResetPassword_Request_Email{
			Email: request.GetEmail(),
		}
	}

	// Saving a new password
	if request.GetToken() != "" && request.GetPassword() != "" {
		request.Request = &pb.ResetPassword_Request_Password{
			Password: request.GetPassword(),
		}
		request.Token = request.GetToken()
	}

	if request.Request == nil && request.GetToken() == "" {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUserHandlersClient(h.Grpc.Client)

	response, err := rClient.ResetPassword(ctx, request)
	if err != nil {
		se, _ := status.FromError(err)
		if se.Message() != "" {
			return httputil.StatusBadRequest(c, se.Message(), nil)
		}
		return httputil.InternalServerError(c, internal.ErrUnexpectedError, nil)
	}

	// send token to send email
	if response.GetToken() != "" {
		mailData := map[string]string{
			"Link": fmt.Sprintf("%s/auth/password_reset/%s", internal.GetString("APP_DSN", "https://app.werbot.com"), response.GetToken()),
		}
		go mail.Send(request.GetEmail(), "Reset password confirmation", "password-reset", mailData)
	}

	return httputil.StatusOK(c, "Password reset", response)
}

// @Summary      Profile information
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200 {object} httputil.HTTPResponse
// @Router       /auth/profile [get]
func (h *handler) getProfile(c *fiber.Ctx) error {
	userParameter := middleware.AuthUser(c)
	return httputil.StatusOK(c, "User information", pb.AuthUserInfo{
		UserId:   userParameter.UserID(""),
		UserRole: userParameter.UserRole(),
		Name:     "Werbot User",
	})
}
