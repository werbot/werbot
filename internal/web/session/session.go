package session

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	golang_jwt "github.com/golang-jwt/jwt/v5"

	accountpb "github.com/werbot/werbot/internal/core/account/proto/account"
	userpb "github.com/werbot/werbot/internal/core/user/proto/user"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/pkg/uuid"
)

// UserParameters encapsulates user-specific parameters.
type UserParameters struct {
	User *accountpb.UserParameters
}

// AuthUser authenticates a user based on the request context.
func AuthUser(c *fiber.Ctx) *UserParameters {
	if user := c.Locals("user"); user != nil {
		return userParameters(c)
	}

	auth := c.Get("Authorization")
	const authScheme = "Bearer "
	if strings.HasPrefix(auth, authScheme) {
		token, err := golang_jwt.Parse(strings.TrimSpace(auth[len(authScheme):]), jwt.CustomKeyFunc())
		if err == nil && token.Valid {
			c.Locals("user", token)
			return userParameters(c)
		}
	}

	return &UserParameters{
		User: &accountpb.UserParameters{
			Roles: userpb.Role(userpb.Role_role_unspecified),
		},
	}
}

// userParameters extracts user parameters from the Fiber context.
// It retrieves the JWT token, extracts claims, and maps them to UserParameters.
func userParameters(c *fiber.Ctx) *UserParameters {
	user := c.Locals("user").(*golang_jwt.Token)
	claims := user.Claims.(golang_jwt.MapClaims)

	context := claims["User"].(map[string]any)
	role := userpb.Role(context["roles"].(float64))
	sessionID := claims["sub"].(string)

	return &UserParameters{
		User: &accountpb.UserParameters{
			UserId:    context["user_id"].(string),
			Roles:     role,
			SessionId: sessionID,
		},
	}
}

// UserID returns the provided input string if it is not empty and the user has an admin role.
// Otherwise, it returns the user's ID.
func (u *UserParameters) UserID(input string) string {
	if u.IsUserAdmin() && uuid.IsValid(input) {
		return input
	}
	return u.User.GetUserId()
}

// OriginalUserID returns the original user ID.
func (u *UserParameters) OriginalUserID() string {
	return u.User.GetUserId()
}

// UserRole returns the role of the user.
func (u *UserParameters) UserRole() userpb.Role {
	return u.User.GetRoles()
}

// UserSub returns the subject identifier for the user.
func (u *UserParameters) SessionId() string {
	return u.User.GetSessionId()
}

// IsUserAdmin checks if the user has an admin role.
func (u *UserParameters) IsUserAdmin() bool {
	return u.User.GetRoles() == userpb.Role_admin
}
