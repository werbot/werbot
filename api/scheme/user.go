package scheme

import (
	"github.com/gofiber/fiber/v2"

	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Retrieve profile schemes
// @Description Get a list of schemes associated with a profile, supports pagination.
// @Tags schemes
// @Accept json
// @Produce json
// @Param profile_id query string false "Profile UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param scheme_type path string false "Scheme type (server, database, desktop, container, cloud, application)"
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Success 200 {object} webutil.HTTPResponse{result=schemepb.ProfileSchemes_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/schemes/profile/{scheme_type} [get]
func (h *Handler) profileSchemes(c *fiber.Ctx) error {
	pagination := webutil.GetPaginationFromCtx(c)
	sessionData := session.AuthProfile(c)
	request := &schemepb.ProfileSchemes_Request{
		IsAdmin:    sessionData.IsProfileAdmin(),
		ProfileId:  sessionData.ProfileID(c.Query("profile_id")),
		SchemeType: schemeaccesspb.SchemeType(schemeaccesspb.SchemeType_value[c.Params("scheme_type")]),
		Limit:      pagination.Limit,
		Offset:     pagination.Offset,
		SortBy:     `"scheme"."scheme_type":ASC`,
	}

	rClient := schemepb.NewSchemeHandlersClient(h.Grpc)
	schemes, err := rClient.ProfileSchemes(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(schemes)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Profile schemes", result)
}
