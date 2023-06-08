package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/grpc"
	accountpb "github.com/werbot/werbot/internal/grpc/account/proto"
	userpb "github.com/werbot/werbot/internal/grpc/user/proto"
	"github.com/werbot/werbot/internal/mail"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"
)

// @Summary Sign In a user with the given credentials
// @Description Signs in a user with the given credentials
// @Tags auth
// @Accept  json
// @Produce  json
// @Param request body accountpb.SignIn_Request true "Sign In Request"
// @Success 200 {object} jwt.Tokens
// @Failure 400 {object} webutil.ErrorResponse
// @Failure 500 {object} webutil.ErrorResponse
// @Router /signin [post]
func (h *Handler) signIn(c *fiber.Ctx) error {
	request := new(accountpb.SignIn_Request)

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := accountpb.NewAccountHandlersClient(h.Grpc.Client)
	user, err := rClient.SignIn(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	sub := uuid.New().String()
	newToken, err := jwt.New(&accountpb.UserParameters{
		UserName: "TODO",
		UserId:   user.GetUserId(),
		Roles:    user.GetRole(),
		Sub:      sub,
	})
	if err != nil {
		h.log.Error(err).Send()
		return webutil.FromGRPC(c, errors.New("failed to create token"))
	}

	// We write user_id (user.user.userid) in the database, since if Access_key will not know which user to create a new one
	if !jwt.AddToken(h.Redis, sub, user.GetUserId()) {
		return webutil.FromGRPC(c, errors.New("failed to set cache"))
	}

	return webutil.StatusOK(c, "", jwt.Tokens{
		Access:  newToken.Tokens.Access,
		Refresh: newToken.Tokens.Refresh,
	})
}

// @Summary      Sign out from the system
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse
// @Router       /auth/logout [post]
func (h *Handler) logout(c *fiber.Ctx) error {
	userParameter := middleware.AuthUser(c)
	jwt.DeleteToken(h.Redis, userParameter.UserSub())
	return webutil.StatusOK(c, "successfully logged out", nil)
}

// @Summary      Re-creation of new tokens
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        refresh_token body     string true "Refresh token"
// @Success      200           {object} jwt.Tokens
// @Failure      400,404,500   {object} webutil.HTTPResponse
// @Router       /auth/refresh [post]
func (h *Handler) refresh(c *fiber.Ctx) error {
	request := new(jwt.Tokens)

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}
	claims, err := jwt.Parse(request.Refresh)
	if err != nil {
		h.log.Error(err).Send()
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	sub := jwt.GetClaimSub(*claims)
	userID, err := h.Redis.Get(fmt.Sprintf("ref_token:%s", sub)).Result()
	if err != nil {
		h.log.Error(err).Send()
		return webutil.FromGRPC(c, trace.Error(codes.NotFound))
	}

	if !jwt.ValidateToken(h.Redis, sub) {
		return webutil.FromGRPC(c, errors.New("token has been revoked"))
	}
	jwt.DeleteToken(h.Redis, sub)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := userpb.NewUserHandlersClient(h.Grpc.Client)
	user, err := rClient.User(ctx, &userpb.User_Request{
		UserId: userID,
	})
	if err != nil {
		return webutil.FromGRPC(c, errors.New("failed to select user"))
	}

	newToken, err := jwt.New(&accountpb.UserParameters{
		UserName: "Mr Robot",
		UserId:   user.GetUserId(),
		Roles:    user.GetRole(),
		Sub:      sub,
	})
	if err != nil {
		h.log.Error(err).Send()
		return webutil.FromGRPC(c, errors.New("failed to create token"))
	}

	jwt.AddToken(h.Redis, sub, userID)

	tokens := &jwt.Tokens{
		Access:  newToken.Tokens.Access,
		Refresh: newToken.Tokens.Refresh,
	}

	return webutil.StatusOK(c, "", tokens)
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
// @Success      200         {object} webutil.HTTPResponse{data=auth.ResetPassword_Response}
// @Failure      400,500     {object} webutil.HTTPResponse
// @Router       /auth/password_reset [post]
func (h *Handler) resetPassword(c *fiber.Ctx) error {
	request := new(accountpb.ResetPassword_Request)

	if len(c.Body()) > 0 {
		if err := protojson.Unmarshal(c.Body(), request); err != nil {
			//h.log.Error(err).Send()
			return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
		}
	}

	request.Token = c.Params("reset_token")

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	// Sending an email with a verification link
	if request.GetEmail() != "" {
		request.Request = &accountpb.ResetPassword_Request_Email{
			Email: request.GetEmail(),
		}
	}

	// Saving a new password
	if request.GetToken() != "" && request.GetPassword() != "" {
		request.Request = &accountpb.ResetPassword_Request_Password{
			Password: request.GetPassword(),
		}
		request.Token = request.GetToken()
	}

	if request.Request == nil && request.GetToken() == "" {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := accountpb.NewAccountHandlersClient(h.Grpc.Client)
	response, err := rClient.ResetPassword(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// send token to send email
	if response.GetToken() != "" {
		go func() {
			mailData := map[string]string{
				"Link": fmt.Sprintf("%s/auth/password_reset/%s", internal.GetString("APP_DSN", "http://localhost:5173"), response.GetToken()),
			}
			err := mail.Send(request.GetEmail(), "reset password confirmation", "password-reset", mailData)
			if err != nil {
				h.log.Error(err).Send()
			}
		}()
	}

	return webutil.StatusOK(c, "password reset", response)
}

// @Summary      Profile information
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse
// @Router       /auth/profile [get]
func (h *Handler) getProfile(c *fiber.Ctx) error {
	userParameter := middleware.AuthUser(c)
	return webutil.StatusOK(c, "user information", accountpb.SignIn_Response{
		UserId:   userParameter.UserID(""),
		UserRole: userParameter.UserRole(),
		Name:     "Mr Robot",
	})
}
