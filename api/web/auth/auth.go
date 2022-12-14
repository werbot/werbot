package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/mail"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"

	pb "github.com/werbot/werbot/api/proto/user"
)

// @Summary      Authorization in the system
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        email    body     pb.SignIn_Request true "Email"
// @Param        password body     pb.SignIn_Request true "Password"
// @Success      200      {object} jwt.Tokens
// @Failure      400,500  {object} webutil.HTTPResponse
// @Router       /auth/signin [post]
func (h *Handler) signIn(c *fiber.Ctx) error {
	request := new(pb.SignIn_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.SignIn_RequestMultiError) {
			e := err.(pb.SignIn_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewUserHandlersClient(h.Grpc.Client)
	user, err := rClient.SignIn(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	sub := uuid.New().String()
	newToken, err := jwt.New(&pb.UserParameters{
		UserName: "TODO",
		UserId:   user.GetUserId(),
		Roles:    user.GetRole(),
		Sub:      sub,
	})
	if err != nil {
		h.log.Error(err).Send()
		return webutil.InternalServerError(c, msgFailedToCreateToken, nil)
	}

	// We write user_id (user.user.userid) in the database, since if Access_key will not know which user to create a new one
	if !jwt.AddToken(h.Cache, sub, user.GetUserId()) {
		return webutil.InternalServerError(c, msgFailedToSetCache, nil)
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
	jwt.DeleteToken(h.Cache, userParameter.UserSub())
	return webutil.StatusOK(c, msgSuccessLoggedOut, nil)
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
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	claims, err := jwt.Parse(request.Refresh)
	if err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateParams, nil)
	}

	sub := jwt.GetClaimSub(*claims)
	userID, err := h.Cache.Get(fmt.Sprintf("ref_token::%s", sub))
	if err != nil {
		h.log.Error(err).Send()
		return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	}

	if !jwt.ValidateToken(h.Cache, sub) {
		return webutil.StatusBadRequest(c, msgTokenHasBeenRevoked, nil)
	}
	jwt.DeleteToken(h.Cache, sub)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewUserHandlersClient(h.Grpc.Client)
	user, err := rClient.User(ctx, &pb.User_Request{
		UserId: userID,
	})
	if err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, msgFailedToSelectUser, nil)
	}

	newToken, err := jwt.New(&pb.UserParameters{
		UserName: "Mr Robot",
		UserId:   user.GetUserId(),
		Roles:    user.GetRole(),
		Sub:      sub,
	})
	if err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, msgFailedToCreateToken, nil)
	}

	jwt.AddToken(h.Cache, sub, userID)

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
// @Success      200         {object} webutil.HTTPResponse{data=user.ResetPassword_Response}
// @Failure      400,500     {object} webutil.HTTPResponse
// @Router       /auth/password_reset [post]
func (h *Handler) resetPassword(c *fiber.Ctx) error {
	request := new(pb.ResetPassword_Request)

	if err := protojson.Unmarshal(c.Body(), request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateParams, nil)
	}

	request.Token = c.Params("reset_token")

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.ResetPassword_RequestMultiError) {
			e := err.(pb.ResetPassword_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
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
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewUserHandlersClient(h.Grpc.Client)
	response, err := rClient.ResetPassword(ctx, request)
	if err != nil {
		h.log.Error(err).Send()
		return webutil.InternalServerError(c, strings.ToLower(utils.StatusMessage(500)), nil)
	}

	// send token to send email
	if response.GetToken() != "" {
		mailData := map[string]string{
			"Link": fmt.Sprintf("%s/auth/password_reset/%s", internal.GetString("APP_DSN", "https://app.werbot.com"), response.GetToken()),
		}
		go mail.Send(request.GetEmail(), "reset password confirmation", "password-reset", mailData)
	}

	return webutil.StatusOK(c, msgPasswordReset, response)
}

// @Summary      Profile information
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200 {object} webutil.HTTPResponse
// @Router       /auth/profile [get]
func (h *Handler) getProfile(c *fiber.Ctx) error {
	userParameter := middleware.AuthUser(c)
	return webutil.StatusOK(c, msgUserInfo, pb.AuthUserInfo{
		UserId:   userParameter.UserID(""),
		UserRole: userParameter.UserRole(),
		Name:     "Mr Robot",
	})
}
