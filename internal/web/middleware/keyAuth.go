package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/keyauth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	projectpb "github.com/werbot/werbot/internal/core/project/proto/project"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// KeyMiddleware handles API key authentication.
type KeyMiddleware struct {
	grpcClient *grpc.ClientConn
}

// Key initializes the KeyMiddleware with a gRPC client connection.
func Key(grpcClient *grpc.ClientConn) *KeyMiddleware {
	return &KeyMiddleware{
		grpcClient: grpcClient,
	}
}

// Execute protects routes using KeyAuth middleware.
func (m KeyMiddleware) Execute() fiber.Handler {
	return keyauth.New(keyauth.Config{
		SuccessHandler: keySuccess,
		ErrorHandler:   keyError,
		KeyLookup:      "header:x-api-key",
		Validator:      m.tokenCheck,
	})
}

// keyError handles key authentication errors.
func keyError(c *fiber.Ctx, _ error) error {
	return webutil.FromGRPC(c, status.Error(codes.Unauthenticated, "Invalid API key"))
}

// keySuccess handles successful key authentication.
func keySuccess(c *fiber.Ctx) error {
	return c.Next()
}

// tokenCheck validates the provided token by checking it against the gRPC service.
func (m KeyMiddleware) tokenCheck(c *fiber.Ctx, token string) (bool, error) {
	rClient := projectpb.NewProjectHandlersClient(m.grpcClient)
	project, err := rClient.ProjectKey(c.UserContext(), &projectpb.ProjectKey_Request{
		Type: &projectpb.ProjectKey_Request_Public{
			Public: &projectpb.ProjectKey_Public{
				Key: token,
			},
		},
	})
	if err != nil {
		return false, err
	}

	c.Set("project-id", project.ProjectId)
	return true, nil
}
