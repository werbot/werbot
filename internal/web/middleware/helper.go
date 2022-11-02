package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"

	pb "github.com/werbot/werbot/internal/grpc/proto/user"
)

// UserParameters is ...
type UserParameters struct {
	User *pb.UserParameters
}

// GetUserParameters is ...
func GetUserParameters(c *fiber.Ctx) *UserParameters {
	if c.Locals("user") != nil {
		user := c.Locals("user").(*jwt.Token)

		context := user.Claims.(jwt.MapClaims)["context"].(map[string]any)
		userRole := context["user_role"].(float64)
		sub := user.Claims.(jwt.MapClaims)["sub"].(string)

		return &UserParameters{
			User: &pb.UserParameters{
				UserId:   context["user_id"].(string),
				UserRole: pb.RoleUser(userRole),
				Sub:      sub,
			},
		}
	}

	return &UserParameters{
		User: &pb.UserParameters{
			UserRole: pb.RoleUser(pb.RoleUser_ROLE_USER_UNSPECIFIED),
		},
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
