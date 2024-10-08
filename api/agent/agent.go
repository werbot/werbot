package agent

import (
	"github.com/gofiber/fiber/v2"

	agentpb "github.com/werbot/werbot/internal/grpc/agent/proto/agent"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// authToken
func (h *Handler) authToken(c *fiber.Ctx) error {
	request := &agentpb.Auth_Request{
		Token: c.Params("token"),
	}

	rClient := agentpb.NewAgentHandlersClient(h.Grpc)
	keys, err := rClient.Auth(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(keys)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Auth data", result)
}

// @Summary      Adding a new scheme
// @Tags         schemes
// @Accept       json
// @Produce      json
// @Param        req         body     schemepb.AddScheme_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=schemepb.AddScheme_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/service/server [post]
func (h *Handler) addScheme(c *fiber.Ctx) error {
	request := &agentpb.AddScheme_Request{
		Token: c.Params("token"),
	}

	_ = webutil.Parse(c, request).Body()

	rClient := agentpb.NewAgentHandlersClient(h.Grpc)
	scheme, err := rClient.AddScheme(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(scheme)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Scheme data", result)
}

// AgentUpdateAutoScheme is ...
//func (h *Handler) updateScheme(c *fiber.Ctx) error {
//	return webutil.StatusOK(c, "Status", "online")
//}
