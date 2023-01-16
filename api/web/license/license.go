package license

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	licensepb "github.com/werbot/werbot/api/proto/license"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"
)

// @Summary      Information about the license currently in use
// @Tags         license
// @Accept       json
// @Produce      json
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/license/info [get]
func (h *Handler) getLicenseInfo(c *fiber.Ctx) error {
	userParameter := middleware.AuthUser(c)

	if !userParameter.IsUserAdmin() {
		return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := licensepb.NewLicenseHandlersClient(h.Grpc.Client)
	lic, err := rClient.License(ctx, &licensepb.License_Request{})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgLicenseInfo, lic)
}
