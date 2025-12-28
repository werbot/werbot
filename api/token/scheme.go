package token

import (
	"github.com/gofiber/fiber/v2"

	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

func (h *Handler) schemeTokens(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	pagination := webutil.GetPaginationFromCtx(c)

	request := &tokenmessage.SchemeTokens_Request{
		IsAdmin:  sessionData.IsProfileAdmin(),
		OwnerId:  sessionData.ProfileID(c.Query("owner_id")),
		SchemeId: sessionData.ProfileID(c.Query("scheme_id")),
		Limit:    pagination.Limit,
		Offset:   pagination.Offset,
		SortBy:   "created_at:ASC",
	}

	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
	tokens, err := rClient.SchemeTokens(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(tokens)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Scheme tokens", result)
}

func (h *Handler) addSchemeToken(c *fiber.Ctx) error {
	return nil
}

func (h *Handler) updateSchemeToken(c *fiber.Ctx) error {
	return nil
}

func (h *Handler) deleteSchemeToken(c *fiber.Ctx) error {
	return nil
}
