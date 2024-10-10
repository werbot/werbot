package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/event"
	accountpb "github.com/werbot/werbot/internal/core/account/proto/account"
	userpb "github.com/werbot/werbot/internal/core/user/proto/user"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
	"github.com/werbot/werbot/pkg/uuid"
)

// @Summary Sign in a user
// @Description Authenticates a user and returns JWT tokens
// @Tags auth
// @Accept json
// @Produce json
// @Param body body accountpb.SignIn_Request true "Sign In Request"
// @Success 200 {object} webutil.HTTPResponse{result=accountpb.Tokens}
// @Failure 400, 500 {object} webutil.HTTPResponse{result=string}
// @Router /auth/signin [post]
func (h *Handler) signIn(c *fiber.Ctx) error {
	request := &accountpb.SignIn_Request{}

	_ = webutil.Parse(c, request).Body()

	rClient := accountpb.NewAccountHandlersClient(h.Grpc)
	user, err := rClient.SignIn(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	sessionID := uuid.New()
	jwtConfig, err := jwt.New(&accountpb.UserParameters{
		UserName:  user.GetName(),
		UserId:    user.GetUserId(),
		Roles:     user.GetRole(),
		SessionId: sessionID,
	})
	if err != nil {
		return webutil.StatusInternalServerError(c, "JWT configuration issue")
	}

	newToken, err := jwtConfig.PairTokens()
	if err != nil {
		return webutil.StatusInternalServerError(c, "Failed to create token")
	}

	// Adding access and refresh tokens to the cache
	cacheData := jwt.SessionInfo{UserID: user.GetUserId()}
	for _, token := range []string{"access_token", "refresh_token"} {
		jwt.CacheAdd(h.Redis, token, sessionID, cacheData)
	}

	// Log the event
	sessionData := &session.UserParameters{
		User: &accountpb.UserParameters{
			SessionId: sessionID,
			UserId:    user.GetUserId(),
		},
	}
	go event.New(h.Grpc).Web(c, sessionData).Profile(user.GetUserId(), event.ProfileProfile, event.OnLogin)

	return webutil.StatusOK(c, "Tokens", accountpb.Token_Response{
		Access:  newToken.Access,
		Refresh: newToken.Refresh,
	})
}

// @Summary Log out a user
// @Description Logs out a user by invalidating their JWT tokens
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {object} webutil.HTTPResponse{result=string}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /auth/logout [post]
func (h *Handler) logout(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)

	for _, token := range []string{"access_token", "refresh_token"} {
		jwt.CacheDelete(h.Redis, token, sessionData.SessionId())
	}

	// Log the event
	go event.New(h.Grpc).Web(c, sessionData).Profile(sessionData.UserID(""), event.ProfileProfile, event.OnLogoff)

	return webutil.StatusOK(c, "Successful logout", nil)
}

// @Summary Refresh JWT tokens
// @Description Refreshes the access and refresh tokens for a user
// @Tags auth
// @Accept json
// @Produce json
// @Param body body jwt.Tokens true "Tokens"
// @Success 200 {object} webutil.HTTPResponse{result=accountpb.Tokens}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /auth/refresh [post]
func (h *Handler) refresh(c *fiber.Ctx) error {
	request := &accountpb.Token_Request{}

	_ = webutil.Parse(c, request).Body()

	claimsRefresh, err := jwt.Parse(request.Refresh)
	if err != nil {
		return webutil.StatusBadRequest(c, "Impossible to parse the key")
	}

	sessionID, err := claimsRefresh.GetSubject()
	if err != nil {
		return webutil.StatusInternalServerError(c, "Impossible to read the key")
	}

	sessionData, err := jwt.CacheGet(h.Redis, "refresh_token", sessionID)
	if err != nil {
		return webutil.StatusUnauthorized(c, "The token has been revoked")
	}

	user, err := userpb.NewUserHandlersClient(h.Grpc).User(c.UserContext(), &userpb.User_Request{
		UserId: sessionData.UserID,
	})
	if err != nil {
		return webutil.StatusInternalServerError(c, "Failed to select user")
	}

	jwtConfig, err := jwt.New(&accountpb.UserParameters{
		UserName:  user.GetName(),
		UserId:    user.GetUserId(),
		Roles:     user.GetRole(),
		SessionId: sessionID,
	})
	if err != nil {
		return webutil.StatusInternalServerError(c, "Failed creates a new instance of Config with initialized keys")
	}

	cacheData := jwt.SessionInfo{UserID: sessionData.UserID}
	jwt.CacheAdd(h.Redis, "access_token", sessionID, cacheData)

	newToken, err := jwtConfig.PairTokens()
	if err != nil {
		return webutil.StatusInternalServerError(c, "Failed to create pair token")
	}

	return webutil.StatusOK(c, "Tokens", accountpb.Token_Response{
		Access:  newToken.Access,
		Refresh: newToken.Refresh,
	})
}

// @Summary Reset user password
// @Description Initiates or completes the password reset process
// @Tags auth
// @Accept json
// @Produce json
// @Param body body accountpb.ResetPassword_Request true "Reset Password Request"
// @Success 200 {object} webutil.HTTPResponse{result=accountpb.ResetPassword_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /auth/password_reset [post]
func (h *Handler) resetPassword(c *fiber.Ctx) error {
	request := &accountpb.ResetPassword_Request{}

	_ = webutil.Parse(c, request).Body(true)

	if request.Request == nil {
		return webutil.StatusBadRequest(c, map[string]string{
			"email": "value is not a valid email address",
		})
	}

	rClient := accountpb.NewAccountHandlersClient(h.Grpc)
	response, err := rClient.ResetPassword(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// userID := response.GetUserId()
	// response.UserId = ""
	//result, err := protoutils.ConvertProtoMessageToMap(response)
	//if err != nil {
	//	return webutil.FromGRPC(c, err)
	//}

	// Log the event
	var eventType event.EventType
	switch request.GetRequest().(type) {
	case *accountpb.ResetPassword_Request_Email: // Sending an email with a verification link
		eventType = event.OnReset
	case *accountpb.ResetPassword_Request_Password: // Saving a new password
		eventType = event.OnUpdate
	default:
		eventType = event.Unspecified
	}

	// Log the event
	sessionData := &session.UserParameters{
		User: &accountpb.UserParameters{
			SessionId: "00000000-0000-0000-0000-000000000000",
			UserId:    response.GetUserId(),
		},
	}
	go event.New(h.Grpc).Web(c, sessionData).Profile(response.GetUserId(), event.ProfilePassword, eventType)

	return webutil.StatusOK(c, "Password reset", nil)
}

// @Summary Check reset token
// @Description Verifies if the provided password reset token is valid
// @Tags auth
// @Produce json
// @Param reset_token path string true "Reset Token"
// @Success 200 {object} webutil.HTTPResponse{result=accountpb.ResetPassword_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /auth/password_reset/{reset_token} [get]
func (h *Handler) checkResetToken(c *fiber.Ctx) error {
	request := &accountpb.ResetPassword_Request{
		Request: &accountpb.ResetPassword_Request_Token{
			Token: c.Params("reset_token"),
		},
	}

	rClient := accountpb.NewAccountHandlersClient(h.Grpc)
	response, err := rClient.ResetPassword(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(response)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Check token", result)
}
