package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/keyauth/v2"

	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/pkg/webutil"

	pb "github.com/werbot/werbot/api/proto/project"
)

// KeyMiddleware is ...
type KeyMiddleware struct {
	*grpc.ClientService
}

// Key is ...
func Key(grpc *grpc.ClientService) *KeyMiddleware {
	return &KeyMiddleware{
		ClientService: grpc,
	}
}

// Execute is ...
func (m KeyMiddleware) Execute() fiber.Handler {
	return keyauth.New(keyauth.Config{
		SuccessHandler: keySuccess,
		ErrorHandler:   keyError,
		KeyLookup:      strings.Join([]string{"header", "Token"}, ":"),
		Validator:      m.tokenCheck,
	})
}

func keyError(c *fiber.Ctx, e error) error {
	return webutil.StatusUnauthorized(c, "Invalid or expired API Key", nil)
}

func keySuccess(c *fiber.Ctx) error {
	return c.Next()
}

func (m KeyMiddleware) tokenCheck(c *fiber.Ctx, token string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewProjectHandlersClient(m.Client)

	project, err := rClient.ListProjects(ctx, &pb.ListProjects_Request{
		Query: fmt.Sprintf("api_key='%v'", token),
	})
	if err != nil || project.Total < 1 {
		return false, err
	}

	c.Set("Project-Id", project.Projects[0].ProjectId)
	return true, nil
}
