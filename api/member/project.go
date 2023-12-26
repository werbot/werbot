package member

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/grpc"
	memberpb "github.com/werbot/werbot/internal/grpc/member/proto"
	userpb "github.com/werbot/werbot/internal/grpc/user/proto"
	"github.com/werbot/werbot/internal/mail"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"
)

// @Summary      Show information about member or list of all members on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true  "Project ID"
// @Param        owner_id        path     uuid false "Project owner ID"
// @Param        member_id       path     uuid false "Member ID. Parameter Accessible with ROLE_ADMIN rights"
// @Success      200             {object} webutil.HTTPResponse{data=memberpb.ProjectMember_Response}
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/members [get]
func (h *Handler) getProjectMember(c *fiber.Ctx) error {
	request := new(memberpb.ProjectMember_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInvalidArgument(c)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)

	// show all members
	if request.GetMemberId() == "" {
		pagination := webutil.GetPaginationFromCtx(c)
		sanitizeSQL, _ := sanitize.SQL(`owner_id = $1 AND project_id = $2`,
			request.GetOwnerId(),
			request.GetProjectId(),
		)
		members, err := rClient.ListProjectMembers(ctx, &memberpb.ListProjectMembers_Request{
			Limit:  pagination.GetLimit(),
			Offset: pagination.GetOffset(),
			SortBy: "member_created:DESC",
			Query:  sanitizeSQL,
		})
		if err != nil {
			return webutil.FromGRPC(c, err)
		}
		if members.GetTotal() == 0 {
			return webutil.FromGRPC(c, status.Error(codes.NotFound, "Not found"))
		}

		return webutil.StatusOK(c, "members", members)
	}

	// show information about the member
	member, err := rClient.ProjectMember(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}
	// if member == nil {
	// 	return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	// }

	return webutil.StatusOK(c, "member information", member)
}

// @Summary      Adding a new member on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     memberpb.AddProject_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=memberpb.AddProjectMember_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/members [post]
func (h *Handler) addProjectMember(c *fiber.Ctx) error {
	request := new(memberpb.AddProjectMember_Request)

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())
	request.Role = userpb.Role_user // TODO directly install the role of the user

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	member, err := rClient.AddProjectMember(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "member added", member)
}

// @Summary      Updating member information on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req             body     memberpb.UpdateMember_Request{}
// @Success      200             {object} webutil.HTTPResponse{data=memberpb.UpdateProjectMember_Response}
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/members [patch]
func (h *Handler) updateProjectMember(c *fiber.Ctx) error {
	request := new(memberpb.UpdateProjectMember_Request)

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	if _, err := rClient.UpdateProjectMember(ctx, request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "member updated", nil)
}

// @Summary      Delete member on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true "Project ID"
// @Param        owner_id        path     uuid true "Owner ID"
// @Param        member_id       path     uuid true "Member ID"
// @Success      200             {object} webutil.HTTPResponse
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/members [delete]
func (h *Handler) deleteProjectMember(c *fiber.Ctx) error {
	request := new(memberpb.DeleteProjectMember_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInvalidArgument(c)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	if _, err := rClient.DeleteProjectMember(ctx, request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "member deleted", nil)
}

// @Summary      List users without project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true "Project ID"
// @Param        owner_id        path     uuid true "Owner ID"
// @Param        name            path     string true "Name"
// @Success      200             {object} webutil.HTTPResponse
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/members/search [get]
func (h *Handler) getUsersWithoutProject(c *fiber.Ctx) error {
	request := new(memberpb.UsersWithoutProject_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInvalidArgument(c)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	members, err := rClient.UsersWithoutProject(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "users without project", members)
}

// @Summary      Update member status of project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     memberpb.UpdateProjectMember_Request{data=memberpb.UpdateProjectMember_Request_Active}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/members/active [patch]
func (h *Handler) updateProjectMemberStatus(c *fiber.Ctx) error {
	request := new(memberpb.UpdateProjectMember_Request)
	request.Setting = new(memberpb.UpdateProjectMember_Request_Active)

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	if _, err := rClient.UpdateProjectMember(ctx, request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// message section
	message := "member is online"
	if !request.GetActive() {
		message = "member is offline"
	}

	return webutil.StatusOK(c, message, nil)
}

// @Summary      Show invites on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        owner_id        path     uuid false "Project owner ID"
// @Param        project_id      path     uuid true  "Project ID"
// @Param        member_id       path     uuid true  "Member ID"
// @Success      200             {object} webutil.HTTPResponse{data=memberpb.ListMembersInvite_Response}
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/members/invite [get]
func (h *Handler) getProjectMembersInvite(c *fiber.Ctx) error {
	request := new(memberpb.ListMembersInvite_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInvalidArgument(c)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	pagination := webutil.GetPaginationFromCtx(c)
	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())
	request.SortBy = `"created":ASC`
	request.Limit = pagination.GetLimit()
	request.Offset = pagination.GetOffset()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	members, err := rClient.ListMembersInvite(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "member invites", members)
}

// @Summary      Invite a new member on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     memberpb.AddProjectMemberInvite_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=memberpb.AddMemberInvite_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/members/invite [post]
func (h *Handler) addProjectMemberInvite(c *fiber.Ctx) error {
	request := new(memberpb.AddMemberInvite_Request)

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	member, err := rClient.AddMemberInvite(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	mailData := map[string]string{
		"Link": fmt.Sprintf("%s/invite/project/%s", internal.GetString("APP_DSN", "http://localhost:5173"), member.GetInvite()),
	}
	go mail.Send(request.GetEmail(), "project invitation", "project-invite", mailData)

	return webutil.StatusOK(c, "member invited", member)
}

// @Summary      Delete invite on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true "Project ID"
// @Param        owner_id        path     uuid true "Owner ID"
// @Param        invite_id       path     uuid true "Invite ID"
// @Success      200             {object} webutil.HTTPResponse
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/members/invite [delete]
func (h *Handler) deleteProjectMemberInvite(c *fiber.Ctx) error {
	request := new(memberpb.DeleteMemberInvite_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInvalidArgument(c)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	if _, err := rClient.DeleteMemberInvite(ctx, request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "invite deleted", nil)
}

// @Summary      Confirmation of the invitation to join the project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        invite      path     uuid true "Invite"
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,500     {object} webutil.HTTPResponse
// @Router       /v1/members/invite/:invite [post]
func (h *Handler) postMembersInviteActivate(c *fiber.Ctx) error {
	request := new(memberpb.MemberInviteActivate_Request)
	request.Invite = c.Params("invite")

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.User.GetUserId()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	project, err := rClient.MemberInviteActivate(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "invite confirmed", project)
}
