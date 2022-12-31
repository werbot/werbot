//go:build saas

package info

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/pkg/webutil"

	pb "github.com/werbot/werbot/api/proto/update"
)

// @Summary      Actual versions of components
// @Tags         info
// @Accept       json
// @Produce      json
// @Success      200         {object} webutil.HTTPResponse{data=pb.Update_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/update/version [get]
func (h *handler) getUpdateVersion(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUpdateHandlersClient(h.Grpc.Client)

	updateList, err := rClient.Update(ctx, &pb.Update_Request{})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgCurrentVersions, updateList.Components)
}
