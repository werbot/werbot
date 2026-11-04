package agent

import (
	"github.com/gofiber/fiber/v2"

	agentmessage "github.com/werbot/werbot/internal/core/agent/proto/message"
	agentrpc "github.com/werbot/werbot/internal/core/agent/proto/rpc"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Authenticate an agent token
// @Description Returns the API credentials and scheme type for an agent token.
// @Tags agent
// @Produce json
// @Param token path string true "Agent token UUID"
// @Success 200 {object} webutil.HTTPResponse{result=agentmessage.Auth_Response}
// @Failure 400,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/agent/auth/{token} [post]
func (h *Handler) authToken(c *fiber.Ctx) error {
	request := &agentmessage.Auth_Request{
		Token: c.Params("token"),
	}

	rClient := agentrpc.NewAgentHandlersClient(h.Grpc)
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

// @Summary Add a scheme from an agent
// @Description Registers a scheme using the agent token and connection details.
// @Tags agent
// @Accept json
// @Produce json
// @Param token path string true "Agent token UUID"
// @Param body body agentmessage.AddScheme_Request true "Agent scheme details"
// @Success 200 {object} webutil.HTTPResponse{result=agentmessage.AddScheme_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/agent/scheme/{token} [post]
func (h *Handler) addScheme(c *fiber.Ctx) error {
	request := &agentmessage.AddScheme_Request{
		Token: c.Params("token"),
	}

	_ = webutil.Parse(c, request).Body()

	rClient := agentrpc.NewAgentHandlersClient(h.Grpc)
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
