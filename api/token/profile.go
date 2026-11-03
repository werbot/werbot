package token

import (
	"context"

	"github.com/gofiber/fiber/v2"

	profilemessage "github.com/werbot/werbot/internal/core/profile/proto/message"
	profilerpc "github.com/werbot/werbot/internal/core/profile/proto/rpc"
	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary List profile tokens
// @Description Returns registration invites. Access restricted to admins.
// @Tags tokens
// @Produce json
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Success 200 {object} webutil.HTTPResponse{result=tokenmessage.ProfileTokens_Response}
// @Failure 401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/tokens/profile [get]
func (h *Handler) profileTokens(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)

	// access only for admin
	if !sessionData.IsProfileAdmin() {
		return webutil.StatusNotFound(c, nil)
	}

	pagination := webutil.GetPaginationFromCtx(c)
	request := &tokenmessage.ProfileTokens_Request{
		IsAdmin: sessionData.IsProfileAdmin(),
		Action:  tokenenum.Action_register,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		SortBy:  `"created_at":ASC`,
	}

	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
	tokens, err := rClient.ProfileTokens(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(tokens)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Profile tokens", result)
}

// @Summary Create a profile registration invite
// @Description Creates a registration token for a new profile. Access restricted to admins.
// @Tags tokens
// @Accept json
// @Produce json
// @Param body body addProfileTokenRequest true "Registration invite"
// @Success 200 {object} webutil.HTTPResponse{result=tokenmessage.AddTokenProfileRegistration_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/tokens/profile [post]
func (h *Handler) addProfileToken(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)

	// access only for admin
	if !sessionData.IsProfileAdmin() {
		return webutil.StatusNotFound(c, nil)
	}

	body := &addProfileTokenRequest{}
	if err := c.BodyParser(body); err != nil {
		return webutil.StatusBadRequest(c, nil)
	}

	request := &tokenmessage.AddTokenProfileRegistration_Request{
		Data: &tokenmessage.MetaDataProfile{
			Name:    body.Name,
			Surname: body.Surname,
			Email:   body.Email,
		},
	}

	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
	tokenData, err := rClient.AddTokenProfileRegistration(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(tokenData)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Profile token added", result)
}

// profileByEmail looks up a profile by email.
func (h *Handler) profileByEmail(ctx context.Context, email string) (*profilemessage.Profile_Response, error) {
	rClient := profilerpc.NewProfileHandlersClient(h.Grpc)
	return rClient.ProfileByEmail(ctx, &profilemessage.ProfileByEmail_Request{Email: email})
}

// addProfile creates a profile with an alias derived from the email local part.
func (h *Handler) addProfile(ctx context.Context, email, name, surname, password string) (*profilemessage.AddProfile_Response, error) {
	rClient := profilerpc.NewProfileHandlersClient(h.Grpc)
	return rClient.AddProfile(ctx, &profilemessage.AddProfile_Request{
		Alias:     profileAlias(email),
		Email:     email,
		Name:      name,
		Surname:   surname,
		Active:    true,
		Confirmed: true,
		Password:  password,
	})
}
