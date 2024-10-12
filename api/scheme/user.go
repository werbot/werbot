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
// @Description Get a list of schemes associated with a profile, supports pagination and sorting.
// @Tags schemes
// @Accept json
// @Produce json
// @Param profile_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID". One is server|database|desktop|container|cloud|application
// @Param scheme_type path string true "Scheme Type"
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Success 200 {object} webutil.HTTPResponse{result=schemepb.Schemes_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /profile/schemes/:project_id? [get]
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
