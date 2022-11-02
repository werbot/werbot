package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/keyauth/v2"

	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/web/httputil"

	pb "github.com/werbot/werbot/internal/grpc/proto/project"
)

type keyMiddleware struct {
	grpc *grpc.ClientService
}

// NewKeyMiddleware is ...
func NewKeyMiddleware(grpc *grpc.ClientService) Middleware {
	return keyMiddleware{
		grpc: grpc,
	}
}

// Protected is ...
func (m keyMiddleware) Execute() fiber.Handler {
	return keyauth.New(keyauth.Config{
		SuccessHandler: keySuccess,
		ErrorHandler:   keyError,
		KeyLookup:      strings.Join([]string{"header", "Token"}, ":"),
		Validator:      m.tokenCheck,
	})
}

func keyError(c *fiber.Ctx, e error) error {
	return httputil.StatusUnauthorized(c, "Invalid or expired API Key", nil)
}

func keySuccess(c *fiber.Ctx) error {
	return c.Next()
}

func (m keyMiddleware) tokenCheck(c *fiber.Ctx, token string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewProjectHandlersClient(m.grpc.Client)

	project, err := rClient.ListProjects(ctx, &pb.ListProjects_Request{
		Query: fmt.Sprintf("api_key='%v'", token),
	})
	if err != nil || project.Total < 1 {
		return false, err
	}

	c.Set("Project-Id", project.Projects[0].ProjectId)
	return true, nil
}
