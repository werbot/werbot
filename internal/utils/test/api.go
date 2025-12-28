package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/helmet"

	"github.com/werbot/werbot/api"
	profileenum "github.com/werbot/werbot/internal/core/profile/proto/enum"
	profilemessage "github.com/werbot/werbot/internal/core/profile/proto/message"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

type HeadersTable map[string]string

var (
	BodyInvalidArgument = BodyTable{
		"code":    float64(400),
		"message": "Bad Request",
	}

	BodyUnauthorized = BodyTable{
		"code":    float64(401),
		"message": "Unauthorized",
	}

	BodyNotFound = BodyTable{
		"code":    float64(404),
		"message": "Not Found",
	}

	BodyInternalServerError = BodyTable{
		"code":    float64(500),
		"message": "Internal Server Error",
	}
)

// ApiTable represents a test case structure
type APITable struct {
	Name           string       // The name of the test
	PreWorkHook    func()       // A hook function to be executed before the test
	Method         string       // The HTTP method to use in our call
	Path           string       // The URL path that is being requested
	StatusCode     int          // The expected response status code
	Body           BodyTable    // The expected response body
	RequestBody    BodyTable    // The request body to sent with the request
	RequestHeaders HeadersTable // The headers that are being set for the request
	Headers        HeadersTable // The response headers we want to test on
}

// ApiHandler holds the API handler and services for testing
type APIHandler struct {
	*api.Handler

	Postgres *PostgresService
}

// ProfileInfo holds profile information including tokens
type ProfileInfo struct {
	Tokens    *profilemessage.Token_Response
	ProfileID string `json:"profile_id"`
	Role      profileenum.Role
	SessionID string
}

// Tokens holds admin and profile tokens
type Tokens struct {
	Admin *profilemessage.Token_Response `json:"admin_tokens"`
	User  *profilemessage.Token_Response `json:"user_tokens"`
}

// API sets up the test environment and returns a TestHandler and teardown function
func API(t *testing.T, addonDirs ...string) (*APIHandler, func(t *testing.T)) {
	t.Setenv("ENV_MODE", "test")
	t.Setenv("SECURITY_AES_KEY", "3D6A619811A17396E45D514695DCDA3A") // example key for tests, don't change
	t.Setenv("JWT_PUBLIC_KEY", "../../fixtures/keys/jwt/jwt_public.key")
	t.Setenv("JWT_PRIVATE_KEY", "../../fixtures/keys/jwt/jwt_private.key")

	migrationsDirs := []string{"../../migration"}
	fixturesDirs := []string{"../../fixtures/migration"}
	if len(addonDirs) > 0 {
		for _, dir := range addonDirs {
			migrationsDirs = append(migrationsDirs, dir+"migration")
			fixturesDirs = append(migrationsDirs, dir+"fixtures/migration")
		}
	}

	migrations := append(migrationsDirs, fixturesDirs...)
	pgTest := ServerPostgres(t, migrations...)

	redisTest := ServerRedis(context.Background(), t)

	grpcTest, err := ServerGRPC(context.Background(), t, pgTest, redisTest)
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
		App:     appTest,
		Grpc:    grpcTest.ClientConn,
		Redis:   redisTest.conn,
		Auth:    middleware.Auth(redisTest.conn).Execute(),
		EnvMode: "test",
	}

	return &APIHandler{
			Handler:  webHandler,
			Postgres: pgTest,
		}, func(_ *testing.T) {
			redisTest.Close()
			grpcTest.Close()
		}
}

// GetProfileAuth retrieves profile authentication info
func (h *APIHandler) GetProfileAuth(email, password string) *ProfileInfo {
	tokens := h.getAuthToken(&profilemessage.SignIn_Request{
		Email:    email,
		Password: password,
	})

	sessionData, _ := jwt.Parse(tokens.Access)
	context := sessionData["User"].(map[string]any)

	return &ProfileInfo{
		Tokens:    tokens,
		ProfileID: context["profile_id"].(string),
		Role:      profileenum.Role(context["roles"].(float64)),
		SessionID: sessionData["sub"].(string),
	}
}

func (h *APIHandler) getAuthToken(signIn *profilemessage.SignIn_Request) *profilemessage.Token_Response {
	bodyReader, _ := json.Marshal(signIn)
	req := httptest.NewRequest("POST", "/auth/signin", bytes.NewBuffer(bodyReader))
	req.Header.Set("Content-Type", "application/json")
	res, err := h.App.Test(req, -1)
	if err != nil {
		fmt.Println("Failure : ", err)
	}
	defer res.Body.Close()

	tokens := &profilemessage.Token_Response{}
	body := &webutil.HTTPResponse{
		Result: tokens,
	}
	json.NewDecoder(res.Body).Decode(body)
	return tokens
}

func (h *APIHandler) AddRoute404() {
	h.App.Use(func(c *fiber.Ctx) error {
		return webutil.StatusNotFound(c, nil)
	})
}

// TestProfileAuth is ...
func (h *APIHandler) TestProfileAuth() (adminHeader map[string]string, userHeader map[string]string) {
	adminAuth := h.GetProfileAuth(ConstAdminEmail, ConstAdminPassword)
	adminHeader = map[string]string{"Authorization": "Bearer " + adminAuth.Tokens.Access}

	userAuth := h.GetProfileAuth(ConstUserEmail, ConstUserPassword)
	userHeader = map[string]string{"Authorization": "Bearer " + userAuth.Tokens.Access}

	return adminHeader, userHeader
}
