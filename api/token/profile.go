package token

import (
	"github.com/gofiber/fiber/v2"

	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

func (h *Handler) profileTokens(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	pagination := webutil.GetPaginationFromCtx(c)

	request := &tokenmessage.ProfileTokens_Request{
		IsAdmin: sessionData.IsProfileAdmin(),
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		SortBy:  "created_at:ASC",
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

/*
func (h *Handler) profileToken(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &tokenmessage.Token_Request{
		IsAdmin: sessionData.IsProfileAdmin(),
		Token:   c.Params("token"),
	}

	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
	member, err := rClient.Token(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(member)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Member", result)
}
*/

func (h *Handler) addProfileToken(c *fiber.Ctx) error {
	return nil
}

func (h *Handler) updateProfileToken(c *fiber.Ctx) error {
	return nil
}

func (h *Handler) deleteProfileToken(c *fiber.Ctx) error {
	return nil
}
