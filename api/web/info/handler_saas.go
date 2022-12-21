//go:build saas

package info

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/web/httputil"

	pb "github.com/werbot/werbot/api/proto/update"
)

// @Summary      Actual versions of components
// @Tags         info
// @Accept       json
// @Produce      json
// @Success      200         {object} httputil.HTTPResponse{data=pb.Update_Response}
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/update/version [get]
func (h *handler) getUpdateVersion(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUpdateHandlersClient(h.Grpc.Client)

	updateList, err := rClient.Update(ctx, &pb.Update_Request{})
	if err != nil {
		return httputil.InternalServerError(c, "Unexpected error while getting updates", err)
	}

	return httputil.StatusOK(c, "Actual versions of components", updateList.Components)
}
