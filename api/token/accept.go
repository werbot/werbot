package token

import (
	"github.com/gofiber/fiber/v2"

	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Accept a project or scheme invite
// @Description Resolves or creates the invited profile and grants project or scheme access.
// @Tags tokens
// @Accept json
// @Produce json
// @Param body body acceptRequest true "Invite acceptance body"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/tokens/accept [post]
func (h *Handler) accept(c *fiber.Ctx) error {
	body := &acceptRequest{}
	if err := c.BodyParser(body); err != nil {
		return webutil.StatusBadRequest(c, nil)
	}

	tokenData, err := h.tokenData(c.UserContext(), body.Token)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	section := tokenData.GetSection()
	switch section {
	case tokenenum.Section_project, tokenenum.Section_scheme:
	default:
		return webutil.FromGRPC(c, invalidTokenError())
	}

	if err := validateInviteToken(tokenData, section, tokenenum.Action_request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	profileID, err := h.resolveProfileID(c.UserContext(), tokenData, body.Password, body.Name, body.Surname)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
	switch section {
	case tokenenum.Section_project:
		_, err = rClient.UpdateProjectToken(c.UserContext(), &tokenmessage.UpdateProjectToken_Request{
			IsAdmin:   true,
			Token:     body.Token,
			Status:    tokenenum.Status_done,
			ProfileId: profileID,
		})
	case tokenenum.Section_scheme:
		_, err = rClient.UpdateSchemeToken(c.UserContext(), &tokenmessage.UpdateSchemeToken_Request{
			IsAdmin:   true,
			Token:     body.Token,
			Status:    tokenenum.Status_done,
			ProfileId: profileID,
		})
	}
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Invite accepted", map[string]any{
		"profile_id": profileID,
		"section":    int32(section),
	})
}
