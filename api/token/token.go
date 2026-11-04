package token

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Retrieve token information
// @Description Returns public token details. Secrets are always masked.
// @Tags tokens
// @Produce json
// @Param token path string true "Token UUID"
// @Success 200 {object} webutil.HTTPResponse{result=tokenmessage.Token_Response}
// @Failure 400,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/tokens/{token} [get]
func (h *Handler) token(c *fiber.Ctx) error {
	request := &tokenmessage.Token_Request{
		IsAdmin: false,
		Token:   c.Params("token"),
	}

	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
	tokenData, err := rClient.Token(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(tokenData)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Token", result)
}

// @Summary Delete token
// @Description Marks a token as deleted. The session owner is used unless an admin supplies owner_id.
// @Tags tokens
// @Produce json
// @Param token path string true "Token UUID"
// @Param owner_id query string false "Owner UUID. Accessible with ROLE_ADMIN rights"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/tokens/{token} [delete]
func (h *Handler) deleteToken(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)

	request := &tokenmessage.DeleteToken_Request{
		IsAdmin: sessionData.IsProfileAdmin(),
		OwnerId: sessionData.ProfileID(c.Query("owner_id")),
		Token:   c.Params("token"),
	}

	// DeleteToken compares the token owner with owner_id. Resolve the real owner
	// for an admin who did not pass owner_id explicitly.
	if sessionData.IsProfileAdmin() && c.Query("owner_id") == "" {
		tokenData, err := h.tokenData(c.UserContext(), request.GetToken())
		if err != nil {
			return webutil.FromGRPC(c, err)
		}
		request.OwnerId = tokenData.GetOwnerId()
	}

	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
	if _, err := rClient.DeleteToken(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Token deleted", nil)
}

// tokenData reads a token without masking its identifiers. It is used by the
// public invite flows, which need the real profile/project/scheme ids.
func (h *Handler) tokenData(ctx context.Context, value string) (*tokenmessage.Token_Response, error) {
	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
	return rClient.Token(ctx, &tokenmessage.Token_Request{
		IsAdmin: true,
		Token:   value,
	})
}

// validateInviteToken checks that the token is the expected kind and has not
// been used or revoked yet. Expiration is enforced by the token handler.
func validateInviteToken(tokenData *tokenmessage.Token_Response, section tokenenum.Section, action tokenenum.Action) error {
	if tokenData.GetSection() != section ||
		tokenData.GetAction() != action ||
		tokenData.GetStatus() != tokenenum.Status_sent {
		return invalidTokenError()
	}
	return nil
}

// invalidTokenError returns the standard error for an unusable invite token.
func invalidTokenError() error {
	return status.Error(codes.InvalidArgument, trace.MsgTokenIsInvalid)
}
