package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	golang_jwt "github.com/golang-jwt/jwt/v4"

	accountpb "github.com/werbot/werbot/internal/grpc/account/proto"
	userpb "github.com/werbot/werbot/internal/grpc/user/proto"
	"github.com/werbot/werbot/internal/web/jwt"
)

// UserParameters is ...
type UserParameters struct {
	User *accountpb.UserParameters
}

// AuthUser is ...
func AuthUser(c *fiber.Ctx) *UserParameters {
	if c.Locals("user") != nil {
		return userParameters(c)
	}

	auth := c.Get("Authorization")
	authScheme := "Bearer"
	l := len(authScheme)
	if len(auth) > l+1 && strings.EqualFold(auth[:l], authScheme) {
		token, err := golang_jwt.Parse(strings.TrimSpace(auth[l:]), jwt.CustomKeyFunc())
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

func userParameters(c *fiber.Ctx) *UserParameters {
	user := c.Locals("user").(*golang_jwt.Token)

	context := user.Claims.(golang_jwt.MapClaims)["User"].(map[string]any)
	role := userpb.Role(context["roles"].(float64))
	sub := user.Claims.(golang_jwt.MapClaims)["sub"].(string)

	return &UserParameters{
		User: &accountpb.UserParameters{
			UserId: context["user_id"].(string),
			Roles:  role,
			Sub:    sub,
		},
	}
}

// UserID is ...
func (u *UserParameters) UserID(input string) string {
	userID := u.User.GetUserId()
	if u.User.GetRoles() == userpb.Role_admin && input != "" {
		userID = input
	}
	return userID
}

// OriginalUserID is ...
func (u *UserParameters) OriginalUserID() string {
	return u.User.GetUserId()
}

// UserRole is ...
func (u *UserParameters) UserRole() userpb.Role {
	return u.User.GetRoles()
}

// UserSub is ...
func (u *UserParameters) UserSub() string {
	return u.User.GetSub()
}

// IsUserAdmin is ...
func (u *UserParameters) IsUserAdmin() bool {
	return u.User.GetRoles() == userpb.Role_admin
}
