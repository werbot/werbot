package license

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/api/proto/license"
)

// @Summary      Information about the license currently in use
// @Tags         license
// @Accept       json
// @Produce      json
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/license/info [get]
func (h *handler) getLicenseInfo(c *fiber.Ctx) error {
	userParameter := middleware.AuthUser(c)

	if !userParameter.IsUserAdmin() {
		return httputil.StatusNotFound(c, internal.MsgNotFound, nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewLicenseHandlersClient(h.Grpc.Client)

	lic, err := rClient.License(ctx, &pb.License_Request{})
	if err != nil {
		return httputil.FromGRPC(c, h.log, err)
	}

	return httputil.StatusOK(c, msgLicenseInfo, lic)
}
