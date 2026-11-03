package session

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	golang_jwt "github.com/golang-jwt/jwt/v5"

	profileenum "github.com/werbot/werbot/internal/core/profile/proto/enum"
	profilemessage "github.com/werbot/werbot/internal/core/profile/proto/message"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/pkg/uuid"
)

// ProfileParameters encapsulates profile-specific parameters.
type ProfileParameters struct {
	Profile *profilemessage.ProfileParameters
}

// AuthProfile authenticates a profile based on the request context.
func AuthProfile(c *fiber.Ctx) *ProfileParameters {
	if profile := c.Locals("user"); profile != nil {
		return profileParameters(c)
	}

	auth := c.Get("Authorization")
	const authScheme = "Bearer "
	if strings.HasPrefix(auth, authScheme) {
		token, err := golang_jwt.Parse(strings.TrimSpace(auth[len(authScheme):]), jwt.CustomKeyFunc())
		if err == nil && token.Valid {
			c.Locals("user", token)
			return profileParameters(c)
		}
	}

	return &ProfileParameters{
		Profile: &profilemessage.ProfileParameters{
			Roles: profileenum.Role_role_unspecified,
		},
	}
}

// profileParameters extracts profile parameters from the Fiber context.
// It retrieves the JWT token, extracts claims, and maps them to ProfileParameters.
// Missing or malformed claims yield an unspecified profile instead of panicking,
// so authorization checks reject the request.
func profileParameters(c *fiber.Ctx) *ProfileParameters {
	profile := &profilemessage.ProfileParameters{
		Roles: profileenum.Role_role_unspecified,
	}

	token, ok := c.Locals("user").(*golang_jwt.Token)
	if !ok || token == nil {
		return &ProfileParameters{Profile: profile}
	}

	claims, ok := token.Claims.(golang_jwt.MapClaims)
	if !ok {
		return &ProfileParameters{Profile: profile}
	}

	if sessionID, ok := claims["sub"].(string); ok {
		profile.SessionId = sessionID
	}

	context, ok := claims["User"].(map[string]any)
	if !ok {
		return &ProfileParameters{Profile: profile}
	}

	if profileID, ok := context["profile_id"].(string); ok {
		profile.ProfileId = profileID
	}
	if role, ok := context["roles"].(float64); ok {
		profile.Roles = profileenum.Role(role)
	}

	return &ProfileParameters{Profile: profile}
}

// TokenType returns the token style claim for the parsed JWT stored by the
// authentication middleware. An empty result means the token is not usable.
func TokenType(c *fiber.Ctx) string {
	token, ok := c.Locals("user").(*golang_jwt.Token)
	if !ok || token == nil {
		return ""
	}

	claims, ok := token.Claims.(golang_jwt.MapClaims)
	if !ok {
		return ""
	}

	return jwt.TokenType(claims)
}

// ProfileID returns the provided input string if it is not empty and the profile has an admin role.
// Otherwise, it returns the profile's ID.
func (u *ProfileParameters) ProfileID(input string) string {
	if u.IsProfileAdmin() && uuid.IsValid(input) {
		return input
	}
	return u.Profile.GetProfileId()
}

// OriginalProfileID returns the original profile ID.
func (u *ProfileParameters) OriginalProfileID() string {
	return u.Profile.GetProfileId()
}

// ProfileRole returns the role of the profile.
func (u *ProfileParameters) ProfileRole() profileenum.Role {
	return u.Profile.GetRoles()
}

// SessionId returns the subject identifier for the profile.
// SessionID returns the session id of the profile.
func (u *ProfileParameters) SessionID() string {
	return u.Profile.GetSessionId()
}

// IsProfileAdmin checks if the profile has an admin role.
func (u *ProfileParameters) IsProfileAdmin() bool {
	return u.Profile.GetRoles() == profileenum.Role_admin
}
