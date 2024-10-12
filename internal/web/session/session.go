package session

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	golang_jwt "github.com/golang-jwt/jwt/v5"

	profilepb "github.com/werbot/werbot/internal/core/profile/proto/profile"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/pkg/uuid"
)

// ProfileParameters encapsulates profile-specific parameters.
type ProfileParameters struct {
	Profile *profilepb.ProfileParameters
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
		Profile: &profilepb.ProfileParameters{
			Roles: profilepb.Role(profilepb.Role_role_unspecified),
		},
	}
}

// profileParameters extracts profile parameters from the Fiber context.
// It retrieves the JWT token, extracts claims, and maps them to ProfileParameters.
func profileParameters(c *fiber.Ctx) *ProfileParameters {
	profile := c.Locals("user").(*golang_jwt.Token)
	claims := profile.Claims.(golang_jwt.MapClaims)

	context := claims["User"].(map[string]any)
	role := profilepb.Role(context["roles"].(float64))
	sessionID := claims["sub"].(string)

	return &ProfileParameters{
		Profile: &profilepb.ProfileParameters{
			ProfileId: context["profile_id"].(string),
			Roles:     role,
			SessionId: sessionID,
		},
	}
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
func (u *ProfileParameters) ProfileRole() profilepb.Role {
	return u.Profile.GetRoles()
}

// SessionId returns the subject identifier for the profile.
func (u *ProfileParameters) SessionId() string {
	return u.Profile.GetSessionId()
}

// IsProfileAdmin checks if the profile has an admin role.
func (u *ProfileParameters) IsProfileAdmin() bool {
	return u.Profile.GetRoles() == profilepb.Role_admin
}
