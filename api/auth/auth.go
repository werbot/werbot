package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/grpc"
	accountpb "github.com/werbot/werbot/internal/grpc/account/proto"
	eventpb "github.com/werbot/werbot/internal/grpc/event/proto"
	userpb "github.com/werbot/werbot/internal/grpc/user/proto"
	"github.com/werbot/werbot/internal/mail"
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
	request := &accountpb.SignIn_Request{}

	if err := c.BodyParser(request); err != nil {
		return webutil.StatusBadRequest(c, "The body of the request is damaged")
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := accountpb.NewAccountHandlersClient(h.Grpc)
	user, err := rClient.SignIn(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	idSession := uuid.New().String()
	jwtConfig, err := jwt.New(&accountpb.UserParameters{
		UserName: user.GetName(),
		UserId:   user.GetUserId(),
		Roles:    user.GetRole(),
		Sub:      idSession,
	})
	if err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInternalServerError(c, "JWT configuration issue")
	}

	newToken, err := jwtConfig.PairTokens()
	if err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInternalServerError(c, "Failed to create token")
	}

	// We write user_id (user.user.userid) in the database, since if Access_key will not know which user to create a new one
	cacheData := jwt.SessionInfo{
		UserID: user.GetUserId(),
	}

	// add event in log
	rClientEvent := eventpb.NewEventHandlersClient(h.Grpc)
	_, err = rClientEvent.AddEvent(ctx, &eventpb.AddEvent_Request{
		Section: &eventpb.AddEvent_Request_Profile{
			Profile: &eventpb.Profile{
				Id:      user.GetUserId(),
				Section: eventpb.Profile_profile,
			},
		},
		UserAgent: string(c.Request().Header.UserAgent()),
		Ip:        c.IP(),
		Event:     eventpb.EventType_onLogin,
	})
	if err != nil {
		h.log.Error(err).Send()
	}

	jwt.CacheAdd(h.Redis, "access_token", idSession, cacheData)
	jwt.CacheAdd(h.Redis, "refresh_token", idSession, cacheData)

	return webutil.StatusOK(c, "Tokens", jwt.Tokens{
		Access:  newToken.Access,
		Refresh: newToken.Refresh,
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
	jwt.CacheDelete(h.Redis, "access_token", userParameter.UserSub())
	jwt.CacheDelete(h.Redis, "refresh_token", userParameter.UserSub())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClientEvent := eventpb.NewEventHandlersClient(h.Grpc)
	_, err := rClientEvent.AddEvent(ctx, &eventpb.AddEvent_Request{
		Section: &eventpb.AddEvent_Request_Profile{
			Profile: &eventpb.Profile{
				Id:      userParameter.User.UserId,
				Section: eventpb.Profile_profile,
			},
		},
		UserAgent: string(c.Request().Header.UserAgent()),
		Ip:        c.IP(),
		Event:     eventpb.EventType_onLogoff,
	})
	if err != nil {
		h.log.Error(err).Send()
	}

	return webutil.StatusOK(c, "Successful logout", nil)
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
	request := &jwt.Tokens{}

	if err := c.BodyParser(request); err != nil {
		return webutil.StatusBadRequest(c, "The body of the request is damaged")
	}

	claimsRefresh, err := jwt.Parse(request.Refresh)
	if err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInternalServerError(c, "Impossible to parse the key")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	idSession, err := claimsRefresh.GetSubject()
	if err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInternalServerError(c, "Impossible to read the key")
	}

	sessionData, err := jwt.CacheGet(h.Redis, "refresh_token", idSession)
	if err != nil {
		h.log.Error(err).Send()
		return webutil.StatusUnauthorized(c, "The token has been revoked")
	}

	rClient := userpb.NewUserHandlersClient(h.Grpc)
	user, err := rClient.User(ctx, &userpb.User_Request{
		UserId: sessionData.UserID,
	})
	if err != nil {
		return webutil.StatusInternalServerError(c, "Failed to select user")
	}

	jwtConfig, err := jwt.New(&accountpb.UserParameters{
		UserName: user.GetName(),
		UserId:   user.GetUserId(),
		Roles:    user.GetRole(),
		Sub:      idSession,
	})
	if err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInternalServerError(c, "Failed to create token")
	}

	cacheData := jwt.SessionInfo{
		UserID: sessionData.UserID,
	}
	jwt.CacheAdd(h.Redis, "access_token", idSession, cacheData)

	newToken, _ := jwtConfig.PairTokens()
	tokens := jwt.Tokens{
		Access: newToken.Access,
	}

	// Reissue the token if its lifetime is less than 60 minutes.
	exp, _ := claimsRefresh.GetExpirationTime()
	timeLeft := (exp.Unix() - time.Now().Unix()) / 60 // minuts
	if timeLeft < 60 {
		tokens.Refresh = newToken.Refresh
		jwt.CacheAdd(h.Redis, "refresh_token", idSession, cacheData)
	}

	return webutil.StatusOK(c, "Tokens", tokens)
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
	request := &accountpb.ResetPassword_Request{}

	if len(c.Body()) > 0 {
		if err := protojson.Unmarshal(c.Body(), request); err != nil {
			return webutil.StatusBadRequest(c, "The body of the request is damaged")
		}
	}

	request.Token = c.Params("res_token")
	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err)
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
		return webutil.StatusBadRequest(c, "The body of the request is damaged")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := accountpb.NewAccountHandlersClient(h.Grpc)
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
