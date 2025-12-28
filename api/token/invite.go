package token

import (
	"github.com/gofiber/fiber/v2"

	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

func (h *Handler) token(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &tokenmessage.Token_Request{
		IsAdmin: sessionData.IsProfileAdmin(),
		Token:   c.Params("token_id"),
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

	return webutil.StatusOK(c, "Token", result)
}

func (h *Handler) updateToken(c *fiber.Ctx) error {
	/*
		sessionData := session.AuthProfile(c)
		request := &tokenmessage.Token_Request{
			IsAdmin: sessionData.IsProfileAdmin(),
			Token:   c.Params("token_id"),
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

		return webutil.StatusOK(c, "Token", result)
	*/
	return webutil.StatusOK(c, "Token used", nil)
}
