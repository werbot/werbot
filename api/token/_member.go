package token

/*
import (
	"github.com/gofiber/fiber/v2"

	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// TODO
// @Summary      Confirmation of the invitation to join the project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        invite      path     uuid true "Invite"
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,500     {object} webutil.HTTPResponse
// @Router       /v1/members/invite/:token [get]
func (h *Handler) memberInviteActivate(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &tokenpb.MemberInviteActivate_Request{
		ProfileId: sessionData.ProfileID(c.Query("owner_id")),
		Token:     c.Params("token"),
	}

	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
	project, err := rClient.MemberInviteActivate(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(project)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// TODO depending on the response, redirect to the registration page, authorization or display a message about successful activation

	return webutil.StatusOK(c, "Invite confirmed", result)
}
*/
