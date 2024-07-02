package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/helmet/v2"

	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/api/auth"
	accountpb "github.com/werbot/werbot/internal/grpc/account/proto"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"
)

var (
	BodyUnauthorized = map[string]any{
		"code":    float64(401),
		"message": "Unauthorized",
		//"result":  "Unauthorized",
	}

	BodyNotFound = map[string]any{
		"code":    float64(404),
		"message": "Not Found",
		//"result":  "Not found",
	}

	BodyInvalidArgument = map[string]any{
		"code":    float64(400),
		"message": "Bad Request",
		//"result":  trace.MsgInvalidArgument,
	}
)

// TestCase is ...
type TestCase struct {
	Name          string
	RequestParam  any
	RequestBody   any
	RequestUser   *UserInfo
	RespondBody   func(*http.Response, *http.Request) error
	RespondStatus int
}

// TestHandler is ...
type TestHandler struct {
	*api.Handler

	Postgres *PostgresService
}

// UserInfo is ...
type UserInfo struct {
	Tokens jwt.Tokens
	UserID string `json:"user_id"`
}

// Tokens is ...
type Tokens struct {
	Admin *jwt.Tokens `json:"admin_tokens"`
	User  *jwt.Tokens `json:"user_tokens"`
}

// API is ...
func API(t *testing.T) (*TestHandler, func(t *testing.T)) {
	t.Setenv("JWT_PUBLIC_KEY", "../../fixtures/keys/jwt/jwt_public.key")
	t.Setenv("JWT_PRIVATE_KEY", "../../fixtures/keys/jwt/jwt_private.key")

	pgTest, err := Postgres(t, "../../migration", "../../fixtures/migration")
	if err != nil {
		t.Error(err)
	}

	ctx := context.Background()
	redisTest := Redis(ctx, t)

	grpcTest, err := GRPC(ctx, t, pgTest.conn, redisTest.conn)
	if err != nil {
		t.Error(err)
	}

	appTest := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	appTest.Use(
		cors.New(cors.Config{
			AllowOrigins:  "*",
			AllowMethods:  "GET,POST,HEAD,OPTIONS,PUT,DELETE,PATCH",
			AllowHeaders:  "Origin, Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization,X-Requested-With",
			ExposeHeaders: "Origin",
			// AllowCredentials: true,
		}),
		helmet.New(),
		etag.New(),
	)

	webHandler := &api.Handler{
		App:   appTest,
		Grpc:  grpcTest.ClientConn,
		Redis: redisTest.conn,
		Auth:  middleware.Auth(redisTest.conn).Execute(),
	}

	auth.New(webHandler).Routes()

	return &TestHandler{
			Handler:  webHandler,
			Postgres: pgTest,
		}, func(t *testing.T) {
			pgTest.Close()
			redisTest.Close()
			grpcTest.Close()
		}
}

// GetUserInfo is ...
func (h *TestHandler) GetUserInfo(email, password string) *UserInfo {
	tokens := h.getAuthToken(&accountpb.SignIn_Request{
		Email:    email,
		Password: password,
	})
	return &UserInfo{
		Tokens: *tokens,
		UserID: h.getAuthUserID(tokens.Access),
	}
}

func (h *TestHandler) getAuthToken(signIn *accountpb.SignIn_Request) *jwt.Tokens {
	bodyReader, _ := json.Marshal(signIn)
	req := httptest.NewRequest("POST", "/auth/signin", bytes.NewBuffer(bodyReader))
	req.Header.Set("Content-Type", "application/json")
	res, err := h.App.Test(req, -1)
	if err != nil {
		fmt.Println("Failure : ", err)
	}
	defer res.Body.Close()

	tokens := &jwt.Tokens{}
	body := &webutil.HTTPResponse{
		Result: tokens,
	}
	json.NewDecoder(res.Body).Decode(body)
	return tokens
}

func (h *TestHandler) getAuthUserID(accessToken string) string {
	req, err := http.NewRequest("GET", "/auth/profile", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if err != nil {
		fmt.Println("Failure : ", err)
	}

	res, _ := h.App.Test(req)
	info := map[string]map[string]string{}
	json.NewDecoder(res.Body).Decode(&info)
	return info["result"]["user_id"]
}

func (h *TestHandler) AddRoute404() {
	h.App.Use(func(c *fiber.Ctx) error {
		return webutil.StatusNotFound(c, nil)
	})
}
