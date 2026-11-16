package identity

import (
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
)

// Handler serves the identity Connect services; methods not implemented yet
// answer CodeUnimplemented through the embedded handlers.
type Handler struct {
	werbotv1connect.UnimplementedAuthServiceHandler
	werbotv1connect.UnimplementedProfileServiceHandler
	werbotv1connect.UnimplementedSSHKeyServiceHandler
	werbotv1connect.UnimplementedMemberServiceHandler
	werbotv1connect.UnimplementedTeamServiceHandler
	svc *Service
}

// NewHandler returns the Connect handler for AuthService, ProfileService,
// SSHKeyService, MemberService and TeamService.
func NewHandler(s *Service) *Handler {
	return &Handler{svc: s}
}

// sessionCookie is the Set-Cookie value for token; maxAge < 0 deletes the cookie.
func (h *Handler) sessionCookie(token string, maxAge int) string {
	return newSessionCookie(token, maxAge, h.svc.cfg.SecureCookies).String()
}

// roleToProto maps "owner" to ROLE_OWNER and so on; unknown → ROLE_UNSPECIFIED.
func roleToProto(role string) werbotv1.Role {
	return werbotv1.Role(werbotv1.Role_value["ROLE_"+strings.ToUpper(role)])
}

func userToProto(u User) *werbotv1.User {
	return &werbotv1.User{Id: u.ID.String(), Email: u.Email, Login: u.Login, Name: u.Name, CreatedAt: timestamppb.New(u.CreatedAt)}
}
