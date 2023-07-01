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
	"github.com/werbot/werbot/internal/grpc"
	accountpb "github.com/werbot/werbot/internal/grpc/account/proto"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/internal/web/middleware"
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
	ctx := context.Background()

	pgTest, err := Postgres(t, "../../migration", "../../fixtures/migration")
	if err != nil {
		t.Error(err)
	}

	redisTest := Redis(ctx, t)

	grpcTest, err := GRPC(ctx, t, pgTest.Conn, redisTest.Handler)
	if err != nil {
		t.Error(err)
	}

	appTest := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	appTest.Use(
		cors.New(cors.Config{
			AllowOrigins:     "*",
			AllowMethods:     "GET,POST,HEAD,OPTIONS,PUT,DELETE,PATCH",
			AllowHeaders:     "Origin, Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization,X-Requested-With",
			ExposeHeaders:    "Origin",
			AllowCredentials: true,
		}),
		helmet.New(),
		etag.New(),
	)

	webHandler := &api.Handler{
		App: appTest,
		Grpc: &grpc.ClientService{
			Client: grpcTest.ClientConn,
		},
		Redis: redisTest.Handler,
		Auth:  middleware.Auth(redisTest.Handler).Execute(),
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
	json.NewDecoder(res.Body).Decode(tokens)
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
