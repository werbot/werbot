package token

import (
	"github.com/gofiber/fiber/v2"

	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary List scheme invites
// @Description Returns pending access invitations for a scheme owned by the session profile.
// @Tags tokens
// @Produce json
// @Param scheme_id path string true "Scheme UUID"
// @Param owner_id query string false "Owner UUID. Accessible with ROLE_ADMIN rights"
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Success 200 {object} webutil.HTTPResponse{result=tokenmessage.SchemeTokens_Response}
// @Failure 401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/tokens/scheme/{scheme_id} [get]
func (h *Handler) schemeTokens(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	pagination := webutil.GetPaginationFromCtx(c)

	request := &tokenmessage.SchemeTokens_Request{
		IsAdmin:  true, // the SQL filters by owner; unmask the token id for the owner
		OwnerId:  sessionData.ProfileID(c.Query("owner_id")),
		SchemeId: c.Params("scheme_id"),
		Action:   tokenenum.Action_request,
		Limit:    pagination.Limit,
		Offset:   pagination.Offset,
		SortBy:   `"created_at":ASC`,
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

// @Summary Invite a scheme user
// @Description Grants scheme access to a new profile (email) or an existing profile (profile_id).
// @Tags tokens
// @Accept json
// @Produce json
// @Param scheme_id path string true "Scheme UUID"
// @Param owner_id query string false "Owner UUID. Accessible with ROLE_ADMIN rights"
// @Param body body addSchemeTokenRequest true "Scheme invite"
// @Success 200 {object} webutil.HTTPResponse{result=tokenmessage.AddTokenSchemeAccess_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/tokens/scheme/{scheme_id} [post]
func (h *Handler) addSchemeToken(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)

	body := &addSchemeTokenRequest{}
	if err := c.BodyParser(body); err != nil {
		return webutil.StatusBadRequest(c, nil)
	}

	request := &tokenmessage.AddTokenSchemeAccess_Request{
		OwnerId:  sessionData.ProfileID(c.Query("owner_id")),
		SchemeId: c.Params("scheme_id"),
	}

	if body.Email != "" {
		request.Data = &tokenmessage.AddTokenSchemeAccess_Request_Email{
			Email: body.Email,
		}
	} else {
		request.Data = &tokenmessage.AddTokenSchemeAccess_Request_ProfileId{
			ProfileId: body.ProfileID,
		}
	}

	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
	tokenData, err := rClient.AddTokenSchemeAccess(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(tokenData)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Scheme token added", result)
}
