package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	golang_jwt "github.com/golang-jwt/jwt/v4"

	pb "github.com/werbot/werbot/api/proto/user"
	"github.com/werbot/werbot/internal/web/jwt"
)

// UserParameters is ...
type UserParameters struct {
	User *pb.UserParameters
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
		User: &pb.UserParameters{
			Roles: pb.RoleUser(pb.RoleUser_ROLE_USER_UNSPECIFIED),
		},
	}
}

func userParameters(c *fiber.Ctx) *UserParameters {
	user := c.Locals("user").(*golang_jwt.Token)

	context := user.Claims.(golang_jwt.MapClaims)["User"].(map[string]any)
	role := pb.RoleUser(context["roles"].(float64))
	sub := user.Claims.(golang_jwt.MapClaims)["sub"].(string)

	return &UserParameters{
		User: &pb.UserParameters{
			UserId: context["user_id"].(string),
			Roles:  role,
			Sub:    sub,
		},
	}
}

// UserID is ...
func (u *UserParameters) UserID(input string) string {
	userID := u.User.GetUserId()
	if u.User.GetRoles() == pb.RoleUser_ADMIN && input != "" {
		userID = input
	}
	return userID
}

// OriginalUserID is ...
func (u *UserParameters) OriginalUserID() string {
	return u.User.GetUserId()
}

// UserRole is ...
func (u *UserParameters) UserRole() pb.RoleUser {
	return u.User.GetRoles()
}

// UserSub is ...
func (u *UserParameters) UserSub() string {
	return u.User.GetSub()
}

// IsUserAdmin is ...
func (u *UserParameters) IsUserAdmin() bool {
	return u.User.GetRoles() == pb.RoleUser_ADMIN
}
