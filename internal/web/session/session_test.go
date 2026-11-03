package session_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	golang_jwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	profileenum "github.com/werbot/werbot/internal/core/profile/proto/enum"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/internal/web/session"
)

type authResult struct {
	ProfileID string  `json:"profile_id"`
	Role      int32   `json:"role"`
	SessionID string  `json:"session_id"`
	TokenType string  `json:"token_type"`
	Panic     *string `json:"panic"`
}

func authProfile(t *testing.T, local any) authResult {
	t.Helper()

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/", func(c *fiber.Ctx) (err error) {
		result := &authResult{}

		defer func() {
			if rec := recover(); rec != nil {
				message := "panic"
				result.Panic = &message
			}
			err = c.JSON(*result)
		}()

		if local != nil {
			c.Locals("user", local)
		}

		profile := session.AuthProfile(c)
		result.ProfileID = profile.OriginalProfileID()
		result.Role = int32(profile.ProfileRole())
		result.SessionID = profile.SessionID()
		result.TokenType = session.TokenType(c)

		return nil
	})

	request := httptest.NewRequest("GET", "/", nil)
	response, err := app.Test(request, -1)
	require.NoError(t, err)
	defer response.Body.Close()

	var result authResult
	require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
	return result
}

func TestAuthProfileValidClaims(t *testing.T) {
	result := authProfile(t, &golang_jwt.Token{
		Claims: golang_jwt.MapClaims{
			"sub": "session-id",
			"typ": jwt.TokenTypeAccess,
			"User": map[string]any{
				"profile_id": "profile-id",
				"roles":      float64(profileenum.Role_admin),
			},
		},
	})

	assert.Nil(t, result.Panic)
	assert.Equal(t, "profile-id", result.ProfileID)
	assert.Equal(t, int32(profileenum.Role_admin), result.Role)
	assert.Equal(t, "session-id", result.SessionID)
	assert.Equal(t, jwt.TokenTypeAccess, result.TokenType)
}

func TestAuthProfileMalformedClaimsDoNotPanic(t *testing.T) {
	cases := map[string]any{
		"missing token":      nil,
		"not a token":        "not-a-token",
		"empty claims":       &golang_jwt.Token{Claims: golang_jwt.MapClaims{}},
		"user is not a map":  &golang_jwt.Token{Claims: golang_jwt.MapClaims{"User": "broken"}},
		"claims wrong types": &golang_jwt.Token{Claims: golang_jwt.MapClaims{"User": map[string]any{"profile_id": 12, "roles": "admin"}, "sub": 5}},
		"claims nil map":     &golang_jwt.Token{Claims: nil},
	}

	for name, local := range cases {
		t.Run(name, func(t *testing.T) {
			result := authProfile(t, local)

			assert.Nil(t, result.Panic)
			assert.Empty(t, result.ProfileID)
			assert.Equal(t, int32(profileenum.Role_role_unspecified), result.Role)
			assert.Empty(t, result.SessionID)
			assert.Empty(t, result.TokenType)
		})
	}
}

func TestTokenTypeRejectsMissingClaim(t *testing.T) {
	result := authProfile(t, &golang_jwt.Token{Claims: golang_jwt.MapClaims{"sub": "session-id"}})
	assert.Empty(t, result.TokenType)
}
