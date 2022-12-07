package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/helmet/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/storage/cache"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/internal/web/module/auth"

	pb "github.com/werbot/werbot/internal/grpc/proto/user"
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
	App     *fiber.App
	GRPC    *grpc.ClientService
	Cache   cache.Cache
	Handler http.HandlerFunc
	Auth    *fiber.Handler
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

// InitTestServer is ...
func InitTestServer(envPath string) *TestHandler {
	rand.Seed(time.Now().UnixNano())

	internal.LoadConfig(envPath)

	grpcClient := grpc.NewClient(
		internal.GetString("GRPCSERVER_HOST", "localhost:50051"),
		internal.GetString("GRPCSERVER_TOKEN", "token"),
		internal.GetString("GRPCSERVER_NAMEOVERRIDE", "werbot.com"),
		internal.GetByteFromFile("GRPCSERVER_PUBLIC_KEY", "./grpc_public.key"),
		internal.GetByteFromFile("GRPCSERVER_PRIVATE_KEY", "./grpc_private.key"),
	)

	cacheClient := cache.New(context.TODO(), &redis.Options{
		Addr:     internal.GetString("REDIS_ADDR", "localhost:6379"),
		Password: internal.GetString("REDIS_PASSWORD", "redisPassword"),
	})

	server := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	server.Use(
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

	authMiddleware := middleware.Auth(cacheClient).Execute()
	auth.New(server, grpcClient, cacheClient, authMiddleware).Routes()

	return &TestHandler{
		App:   server,
		GRPC:  grpcClient,
		Cache: cacheClient,
		Auth:  &authMiddleware,
	}
}

// GetUserInfo is ...
func (h *TestHandler) GetUserInfo(signIn *pb.SignIn_Request) *UserInfo {
	tokens := h.getAuthToken(signIn)
	return &UserInfo{
		Tokens: *tokens,
		UserID: h.getAuthUserID(tokens.Access),
	}
}

// FinishHandler is ...
func (h *TestHandler) FinishHandler() {
	h.App.Use(func(c *fiber.Ctx) error {
		return httputil.StatusNotFound(c, internal.ErrNotFound, nil)
	})

	h.Handler = h.fiberToHandlerFunc()
}

func (h *TestHandler) getAuthToken(signIn *pb.SignIn_Request) *jwt.Tokens {
	userData, _ := json.Marshal(signIn)
	req, err := http.NewRequest("POST", "/auth/signin", bytes.NewBuffer(userData))
	req.Header.Add("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		fmt.Println("Failure : ", err)
	}

	res, _ := h.App.Test(req)
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

func (h *TestHandler) fiberToHandlerFunc() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := h.App.Test(r, -1)
		if err != nil {
			panic(err)
		}

		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)

		if _, err := io.Copy(w, resp.Body); err != nil {
			panic(err)
		}
	}
}
