package auth

import (
	"github.com/gofiber/fiber/v2"

	profilepb "github.com/werbot/werbot/internal/core/profile/proto/profile"
	"github.com/werbot/werbot/internal/event"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
	"github.com/werbot/werbot/pkg/uuid"
)

// @Summary Sign in a profile
// @Description Authenticates a profile and returns JWT tokens
// @Tags auth
// @Accept json
// @Produce json
// @Param body body profilepb.SignIn_Request true "Sign In Request"
// @Success 200 {object} webutil.HTTPResponse{result=profilepb.Tokens}
// @Failure 400, 500 {object} webutil.HTTPResponse{result=string}
// @Router /auth/signin [post]
func (h *Handler) signIn(c *fiber.Ctx) error {
	request := &profilepb.SignIn_Request{}

	_ = webutil.Parse(c, request).Body()

	rClient := profilepb.NewProfileHandlersClient(h.Grpc)
	profile, err := rClient.SignIn(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	sessionID := uuid.New()
	jwtConfig, err := jwt.New(&profilepb.ProfileParameters{
		Name:      profile.GetName(),
		ProfileId: profile.GetProfileId(),
		Roles:     profile.GetRole(),
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
	cacheData := jwt.SessionInfo{ProfileID: profile.GetProfileId()}
	for _, token := range []string{"access_token", "refresh_token"} {
		jwt.CacheAdd(h.Redis, token, sessionID, cacheData)
	}

	// Log the event
	sessionData := &session.ProfileParameters{
		Profile: &profilepb.ProfileParameters{
			SessionId: sessionID,
			ProfileId: profile.GetProfileId(),
		},
	}
	go event.New(h.Grpc).Web(c, sessionData).Profile(profile.GetProfileId(), event.ProfileProfile, event.OnLogin)

	return webutil.StatusOK(c, "Tokens", profilepb.Token_Response{
		Access:  newToken.Access,
		Refresh: newToken.Refresh,
	})
}

// @Summary Log out a profile
// @Description Logs out a profile by invalidating their JWT tokens
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {object} webutil.HTTPResponse{result=string}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /auth/logout [post]
func (h *Handler) logout(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)

	for _, token := range []string{"access_token", "refresh_token"} {
		jwt.CacheDelete(h.Redis, token, sessionData.SessionId())
	}

	// Log the event
	go event.New(h.Grpc).Web(c, sessionData).Profile(sessionData.ProfileID(""), event.ProfileProfile, event.OnLogoff)

	return webutil.StatusOK(c, "Successful logout", nil)
}

// @Summary Refresh JWT tokens
// @Description Refreshes the access and refresh tokens for a profile
// @Tags auth
// @Accept json
// @Produce json
// @Param body body jwt.Tokens true "Tokens"
// @Success 200 {object} webutil.HTTPResponse{result=profilepb.Tokens}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /auth/refresh [post]
func (h *Handler) refresh(c *fiber.Ctx) error {
	request := &profilepb.Token_Request{}

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

	profile, err := profilepb.NewProfileHandlersClient(h.Grpc).Profile(c.UserContext(), &profilepb.Profile_Request{
		ProfileId: sessionData.ProfileID,
	})
	if err != nil {
		return webutil.StatusInternalServerError(c, "Failed to select profile")
	}

	jwtConfig, err := jwt.New(&profilepb.ProfileParameters{
		Name:      profile.GetName(),
		ProfileId: profile.GetProfileId(),
		Roles:     profile.GetRole(),
		SessionId: sessionID,
	})
	if err != nil {
		return webutil.StatusInternalServerError(c, "Failed creates a new instance of Config with initialized keys")
	}

	cacheData := jwt.SessionInfo{ProfileID: sessionData.ProfileID}
	jwt.CacheAdd(h.Redis, "access_token", sessionID, cacheData)

	newToken, err := jwtConfig.PairTokens()
	if err != nil {
		return webutil.StatusInternalServerError(c, "Failed to create pair token")
	}

	return webutil.StatusOK(c, "Tokens", profilepb.Token_Response{
		Access:  newToken.Access,
		Refresh: newToken.Refresh,
	})
}

// @Summary Reset profile password
// @Description Initiates or completes the password reset process
// @Tags auth
// @Accept json
// @Produce json
// @Param body body profilepb.ResetPassword_Request true "Reset Password Request"
// @Success 200 {object} webutil.HTTPResponse{result=profilepb.ResetPassword_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /auth/password_reset [post]
func (h *Handler) resetPassword(c *fiber.Ctx) error {
	request := &profilepb.ResetPassword_Request{}

	_ = webutil.Parse(c, request).Body(true)

	if request.Request == nil {
		return webutil.StatusBadRequest(c, map[string]string{
			"email": "value is not a valid email address",
		})
	}

	rClient := profilepb.NewProfileHandlersClient(h.Grpc)
	response, err := rClient.ResetPassword(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// profileID := response.GetProfileId()
	// response.ProfileId = ""
	//result, err := protoutils.ConvertProtoMessageToMap(response)
	//if err != nil {
	//	return webutil.FromGRPC(c, err)
	//}

	// Log the event
	var eventType event.EventType
	switch request.GetRequest().(type) {
	case *profilepb.ResetPassword_Request_Email: // Sending an email with a verification link
		eventType = event.OnReset
	case *profilepb.ResetPassword_Request_Password: // Saving a new password
		eventType = event.OnUpdate
	default:
		eventType = event.Unspecified
	}

	// Log the event
	sessionData := &session.ProfileParameters{
		Profile: &profilepb.ProfileParameters{
			SessionId: "00000000-0000-0000-0000-000000000000",
			ProfileId: response.GetProfileId(),
		},
	}
	go event.New(h.Grpc).Web(c, sessionData).Profile(response.GetProfileId(), event.ProfilePassword, eventType)

	return webutil.StatusOK(c, "Password reset", nil)
}

// @Summary Check reset token
// @Description Verifies if the provided password reset token is valid
// @Tags auth
// @Produce json
// @Param reset_token path string true "Reset Token"
// @Success 200 {object} webutil.HTTPResponse{result=profilepb.ResetPassword_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /auth/password_reset/{reset_token} [get]
func (h *Handler) checkResetToken(c *fiber.Ctx) error {
	request := &profilepb.ResetPassword_Request{
		Request: &profilepb.ResetPassword_Request_Token{
			Token: c.Params("reset_token"),
		},
	}

	rClient := profilepb.NewProfileHandlersClient(h.Grpc)
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
