package middleware

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"

	"github.com/werbot/werbot/internal/config"
	pb "github.com/werbot/werbot/internal/grpc/proto/user"
)

// UserParameters is ...
type UserParameters struct {
	User *pb.UserParameters
}

// GetUserParameters is ...
func GetUserParameters(c *fiber.Ctx) *UserParameters {
	if c.Locals("user") != nil {
		return userParameters(c)
	}

	auth := c.Get("Authorization")
	authScheme := "Bearer"
	l := len(authScheme)
	if len(auth) > l+1 && strings.EqualFold(auth[:l], authScheme) {
		token, err := jwt.Parse(strings.TrimSpace(auth[l:]), customKeyFunc())
		if err == nil && token.Valid {
			c.Locals("user", token)
			return userParameters(c)
		}
	}

	return &UserParameters{
		User: &pb.UserParameters{
			UserRole: pb.RoleUser(pb.RoleUser_ROLE_USER_UNSPECIFIED),
		},
	}
}

func userParameters(c *fiber.Ctx) *UserParameters {
	user := c.Locals("user").(*jwt.Token)

	context := user.Claims.(jwt.MapClaims)["context"].(map[string]any)
	userRole := pb.RoleUser(context["user_role"].(float64))
	sub := user.Claims.(jwt.MapClaims)["sub"].(string)

	return &UserParameters{
		User: &pb.UserParameters{
			UserId:   context["user_id"].(string),
			UserRole: userRole,
			Sub:      sub,
		},
	}
}

func customKeyFunc() jwt.Keyfunc {
	return func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != "HS256" {
			return nil, fmt.Errorf("Unexpected jwt signing method=%v", t.Header["alg"])
		}

		return []byte(config.GetString("ACCESS_TOKEN_SECRET", "secret")), nil
	}
}

// GetUserID is ...
func (u *UserParameters) GetUserID(input string) string {
	userID := u.User.GetUserId()
	if u.User.GetUserRole() == pb.RoleUser_ADMIN && input != "" {
		userID = input
	}
	return userID
}

// GetOriginalUserID is ...
func (u *UserParameters) GetOriginalUserID() string {
	return u.User.GetUserId()
}

// GetUserRole is ...
func (u *UserParameters) GetUserRole() pb.RoleUser {
	return u.User.GetUserRole()
}

// GetUserSub is ...
func (u *UserParameters) GetUserSub() string {
	return u.User.Sub
}

// IsUserAdmin is ...
func (u *UserParameters) IsUserAdmin() bool {
	return u.User.GetUserRole() == pb.RoleUser_ADMIN
}
