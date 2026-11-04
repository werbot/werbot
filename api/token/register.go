package token

import (
	"github.com/gofiber/fiber/v2"

	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Register a profile by invite
// @Description Creates a profile from a registration token and marks the token as done.
// @Tags tokens
// @Accept json
// @Produce json
// @Param body body registerRequest true "Registration body"
// @Success 200 {object} webutil.HTTPResponse{result=map[string]string}
// @Failure 400,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/tokens/register [post]
func (h *Handler) register(c *fiber.Ctx) error {
	body := &registerRequest{}
	if err := c.BodyParser(body); err != nil {
		return webutil.StatusBadRequest(c, nil)
	}

	tokenData, err := h.tokenData(c.UserContext(), body.Token)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	if err := validateInviteToken(tokenData, tokenenum.Section_profile, tokenenum.Action_register); err != nil {
		return webutil.FromGRPC(c, err)
	}

	metadata := tokenData.GetProfile()
	if metadata == nil || metadata.GetEmail() == "" {
		return webutil.FromGRPC(c, invalidTokenError())
	}

	created, err := h.addProfile(c.UserContext(), metadata.GetEmail(), metadata.GetName(), metadata.GetSurname(), body.Password)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
	_, err = rClient.UpdateProfileToken(c.UserContext(), &tokenmessage.UpdateProfileToken_Request{
		IsAdmin:   true,
		Token:     body.Token,
		Status:    tokenenum.Status_done,
		ProfileId: created.GetProfileId(),
	})
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Profile registered", map[string]any{
		"profile_id": created.GetProfileId(),
	})
}
