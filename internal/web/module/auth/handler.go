package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/werbot/werbot/internal/config"
	"github.com/werbot/werbot/internal/logger"
	"github.com/werbot/werbot/internal/message"
	"github.com/werbot/werbot/internal/sender"
	"github.com/werbot/werbot/internal/utils/validator"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/internal/grpc/proto/user"
)

// @Summary      Authorization in the system
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        email    body     pb.AuthUser_Request true "Email"
// @Param        password body     pb.AuthUser_Request true "Password"
// @Success      200      {object} token.Tokens
// @Failure      400,500  {object} httputil.HTTPResponse
// @Router       /auth/signin [post]
func (h *Handler) postSignIn(c *fiber.Ctx) error {
	input := &pb.AuthUser_Request{}
	c.BodyParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUserHandlersClient(h.grpc.Client)

	user, err := rClient.AuthUser(ctx, &pb.AuthUser_Request{
		Email:    input.GetEmail(),
		Password: input.GetPassword(),
	})
	if err != nil {
		se, _ := status.FromError(err)
		if se.Message() != "" {
			return httputil.StatusBadRequest(c, se.Message(), nil)
		}
		return httputil.InternalServerError(c, message.ErrUnexpectedError, nil)
	}

	sub := uuid.New().String()
	newToken, err := httputil.CreateToken(sub, &pb.UserParameters{
		UserId:   user.GetUserId(),
		UserRole: user.GetRole(),
	})
	if err != nil {
		logger.OutErrorLog("gRPC", err, "Failed to create token")
		return httputil.InternalServerError(c, "Failed to create token", nil)
	}

	// We write user_id (user.user.userid) in the database, since if Access_key will not know which user to create a new one
	if err := h.cache.Set(fmt.Sprintf("ref_token::%s", sub), user.GetUserId(), newToken.RefreshTokenExp); err != nil {
		return httputil.InternalServerError(c, "Failed to set cache", nil)
	}

	return httputil.StatusOK(c, "", httputil.Tokens{
		AccessToken:  newToken.Tokens.AccessToken,
		RefreshToken: newToken.Tokens.RefreshToken,
	})
}

// @Summary      Sign out from the system
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200 {object} httputil.HTTPResponse
// @Router       /auth/logout [post]
func (h *Handler) postLogout(c *fiber.Ctx) error {
	userParameter := middleware.GetUserParameters(c)
	h.cache.Delete(fmt.Sprintf("ref_token::%s", userParameter.GetUserSub()))
	return httputil.StatusOK(c, "Successfully logged out", nil)
}

// @Summary      Re-creation of new tokens
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        refresh_token body     string true "Refresh token"
// @Success      200           {object} token.RefreshToken
// @Failure      400,404,500   {object} httputil.HTTPResponse
// @Router       /auth/refresh  [post]
func (h *Handler) postRefresh(c *fiber.Ctx) error {
	input := new(httputil.RefreshToken)
	if err := c.BodyParser(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrBadQueryParams, nil)
	}

	t, err := jwt.Parse(input.Token, httputil.VerifyToken)
	if err != nil {
		return httputil.StatusBadRequest(c, "Token parsing error", nil)
	}
	if _, ok := t.Claims.(jwt.Claims); !ok && !t.Valid {
		return httputil.StatusUnauthorized(c, "Token regeneration error", nil)
	}

	claims, ok := t.Claims.(jwt.MapClaims)
	if ok && t.Valid {
		sub, ok := claims["sub"].(string)
		if !ok {
			return httputil.StatusUnauthorized(c, "Token signature error", nil)
		}

		key := fmt.Sprintf("ref_token::%s", sub)
		userID, err := h.cache.Get(key)
		if err != nil {
			return httputil.StatusUnauthorized(c, "Your token has been revoked", nil)
		}
		h.cache.Delete(key)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		rClient := pb.NewUserHandlersClient(h.grpc.Client)

		user, _ := rClient.GetUser(ctx, &pb.GetUser_Request{
			UserId: userID,
		})

		newToken, err := httputil.CreateToken(sub, &pb.UserParameters{
			UserId:   user.GetUserId(),
			UserRole: user.GetRole(),
		})
		if err != nil {
			return httputil.StatusBadRequest(c, "Failed to create token", nil)
		}

		h.cache.Set(key, userID, newToken.RefreshTokenExp)

		return httputil.StatusOK(c, "", httputil.Tokens{
			AccessToken:  newToken.Tokens.AccessToken,
			RefreshToken: newToken.Tokens.RefreshToken,
		})
	}
	return httputil.StatusUnauthorized(c, "Refresh expired", nil)
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
func (h *Handler) postResetPassword(c *fiber.Ctx) error {
	request := new(pb.ResetPassword_Request)
	if err := protojson.Unmarshal(c.Body(), request); err != nil {
		fmt.Print(err)
	}

	request.Token = c.Params("reset_token")
	if err := validator.ValidateStruct(request); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
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
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUserHandlersClient(h.grpc.Client)

	response, err := rClient.ResetPassword(ctx, request)
	if err != nil {
		se, _ := status.FromError(err)
		if se.Message() != "" {
			return httputil.StatusBadRequest(c, se.Message(), nil)
		}
		return httputil.InternalServerError(c, message.ErrUnexpectedError, nil)
	}

	// send token to send email
	if response.GetToken() != "" {
		mailData := map[string]string{
			"Link": fmt.Sprintf("%s/auth/password_reset/%s", config.GetString("APP_DSN", "https://app.werbot.com"), response.GetToken()),
		}
		go sender.SendMail(request.GetEmail(), "Reset password confirmation", "password-reset", mailData)
	}

	return httputil.StatusOK(c, "Password reset", response)
}

// @Summary      Profile information
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200 {object} httputil.HTTPResponse
// @Router       /auth/profile [get]
func (h *Handler) getProfile(c *fiber.Ctx) error {
	userParameter := middleware.GetUserParameters(c)
	return httputil.StatusOK(c, "User information", pb.AuthUserInfo{
		UserId:   userParameter.GetUserID(""),
		UserRole: userParameter.GetUserRole(),
		Name:     "Werbot User",
	})
}
