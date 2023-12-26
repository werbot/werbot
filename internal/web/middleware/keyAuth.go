package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/keyauth/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	projectpb "github.com/werbot/werbot/internal/grpc/project/proto"
	"github.com/werbot/werbot/pkg/webutil"
)

// KeyMiddleware is ...
type KeyMiddleware struct {
	*grpc.ClientConn
}

// Key is ...
func Key(grpc *grpc.ClientConn) *KeyMiddleware {
	return &KeyMiddleware{
		grpc,
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
	return webutil.FromGRPC(c, status.Error(codes.Unauthenticated, "Invalid or expired API Key"))
}

func keySuccess(c *fiber.Ctx) error {
	return c.Next()
}

func (m KeyMiddleware) tokenCheck(c *fiber.Ctx, token string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := projectpb.NewProjectHandlersClient(m)
	project, err := rClient.ListProjects(ctx, &projectpb.ListProjects_Request{
		Query: fmt.Sprintf("api_key='%v'", token),
	})
	if err != nil || project.Total < 1 {
		return false, err
	}

	c.Set("Project-Id", project.Projects[0].ProjectId)
	return true, nil
}
