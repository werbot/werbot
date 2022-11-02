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

	"github.com/werbot/werbot/internal/cache"
	"github.com/werbot/werbot/internal/config"
	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/message"
	"github.com/werbot/werbot/internal/web/httputil"
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
}

// UserInfo is ...
type UserInfo struct {
	Tokens httputil.Tokens
	UserID string `json:"user_id"`
}

// Tokens is ...
type Tokens struct {
	Admin *httputil.Tokens `json:"admin_tokens"`
	User  *httputil.Tokens `json:"user_tokens"`
}

// InitTestServer is ...
func InitTestServer(envPath string) *TestHandler {
	rand.Seed(time.Now().UnixNano())

	config.Load(envPath)

	grpcClient := grpc.NewClient(
		config.GetString("GRPCSERVER_DSN", "localhost:50051"),
		config.GetString("GRPCSERVER_TOKEN", "token"),
		config.GetString("GRPCSERVER_NAMEOVERRIDE", "werbot.com"),
		config.GetByteFromFile("GRPCSERVER_PUBLIC_KEY", "./grpc_public.key"),
		config.GetByteFromFile("GRPCSERVER_PRIVATE_KEY", "./grpc_private.key"),
	)

	cacheClient := cache.NewRedisClient(context.TODO(), &redis.Options{
		Addr:     config.GetString("REDIS_ADDR", "localhost:6379"),
		Password: config.GetString("REDIS_PASSWORD", ""),
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

	auth.NewHandler(server, grpcClient, cacheClient).Routes()

	return &TestHandler{
		App:   server,
		GRPC:  grpcClient,
		Cache: cacheClient,
	}
}

// GetUserInfo is ...
func (h *TestHandler) GetUserInfo(authUser *pb.AuthUser_Request) *UserInfo {
	tokens := h.getAuthToken(authUser)
	return &UserInfo{
		Tokens: *tokens,
		UserID: h.getAuthUserID(tokens.AccessToken),
	}
}

// FinishHandler is ...
func (h *TestHandler) FinishHandler() {
	h.App.Use(func(c *fiber.Ctx) error {
		return httputil.StatusNotFound(c, message.ErrNotFound, nil)
	})

	h.Handler = h.fiberToHandlerFunc()
}

func (h *TestHandler) getAuthToken(authUser *pb.AuthUser_Request) *httputil.Tokens {
	userData, _ := json.Marshal(authUser)
	req, err := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(userData))
	req.Header.Add("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		fmt.Println("Failure : ", err)
	}

	res, _ := h.App.Test(req)
	tokens := &httputil.Tokens{}
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
