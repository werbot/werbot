package license

import (
	"github.com/gofiber/fiber/v2"

	licenserpc "github.com/werbot/werbot/internal/core/license/proto/rpc"
	licensemessage "github.com/werbot/werbot/internal/core/license/proto/message"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Get License Information
// @Description Retrieve the license information for the authenticated profile
// @Tags license
// @Produce json
// @Success 200 {object} webutil.HTTPResponse{result=licensemessage.License_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/license/info  [get]
func (h *Handler) licenseInfo(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	if !sessionData.IsProfileAdmin() {
		return webutil.StatusNotFound(c, nil)
	}

	rClient := licenserpc.NewLicenseHandlersClient(h.Grpc)
	lic, err := rClient.License(c.UserContext(), &licensemessage.License_Request{})
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(lic)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "License", result)
}
