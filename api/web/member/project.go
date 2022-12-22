package member

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/mail"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/utils/validate"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/api/proto/member"
	pb_user "github.com/werbot/werbot/api/proto/user"
)

// @Summary      Show information about member or list of all members on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true  "Project ID"
// @Param        owner_id        path     uuid false "Project owner ID"
// @Param        member_id       path     uuid false "Member ID. Parameter Accessible with ROLE_ADMIN rights"
// @Success      200             {object} httputil.HTTPResponse{data=pb.ProjectMember_Response}
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members [get]
func (h *handler) getProjectMember(c *fiber.Ctx) error {
	input := new(pb.ProjectMember_Request)

	if err := c.QueryParser(input); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	userParameter := middleware.AuthUser(c)
	ownerID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

	// show all members
	if input.GetMemberId() == "" {
		pagination := httputil.GetPaginationFromCtx(c)
		sanitizeSQL, _ := sanitize.SQL(`owner_id = $1 AND project_id = $2`, ownerID, input.GetProjectId())
		members, err := rClient.ListProjectMembers(ctx, &pb.ListProjectMembers_Request{
			Limit:  pagination.GetLimit(),
			Offset: pagination.GetOffset(),
			SortBy: "member_created:DESC",
			Query:  sanitizeSQL,
		})
		if err != nil {
			return httputil.ErrorGRPC(c, h.log, err)
		}
		if members.GetTotal() == 0 {
			return httputil.StatusNotFound(c, internal.MsgNotFound, nil)
		}

		return httputil.StatusOK(c, "Members", members)
	}

	// show information about the member
	member, err := rClient.ProjectMember(ctx, &pb.ProjectMember_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		MemberId:  input.GetMemberId(),
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}
	// if member == nil {
	// 	return httputil.StatusNotFound(c, internal.MsgNotFound, nil)
	// }

	return httputil.StatusOK(c, "Member information", member)
}

// @Summary      Adding a new member on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     pb.AddProject_Request{}
// @Success      200         {object} httputil.HTTPResponse{data=pb.AddProjectMember_Response}
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/members [post]
func (h *handler) addProjectMember(c *fiber.Ctx) error {
	input := new(pb.AddProjectMember_Request)

	if err := c.BodyParser(input); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

	member, err := rClient.AddProjectMember(ctx, &pb.AddProjectMember_Request{
		OwnerId:   userID,
		ProjectId: input.GetProjectId(),
		UserId:    input.GetUserId(),
		Role:      pb_user.RoleUser_USER, // TODO directly install the role of the user
		Active:    input.GetActive(),
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}

	return httputil.StatusOK(c, "Member added", member)
}

// @Summary      Updating member information on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req             body     pb.UpdateMember_Request{}
// @Success      200             {object} httputil.HTTPResponse{data=pb.UpdateProjectMember_Response}
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members [patch]
func (h *handler) patchProjectMember(c *fiber.Ctx) error {
	input := new(pb.UpdateProjectMember_Request)

	if err := c.BodyParser(input); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	userParameter := middleware.AuthUser(c)
	ownerID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

	_, err := rClient.UpdateProjectMember(ctx, &pb.UpdateProjectMember_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		MemberId:  input.GetMemberId(),
		Role:      input.GetRole(),
		Active:    input.GetActive(),
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}

	return httputil.StatusOK(c, "Member updated", nil)
}

// @Summary      Delete member on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true "Project ID"
// @Param        owner_id        path     uuid true "Owner ID"
// @Param        member_id       path     uuid true "Member ID"
// @Success      200             {object} httputil.HTTPResponse
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members [delete]
func (h *handler) deleteProjectMember(c *fiber.Ctx) error {
	input := new(pb.DeleteProjectMember_Request)

	if err := c.QueryParser(input); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	userParameter := middleware.AuthUser(c)
	ownerID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

	_, err := rClient.DeleteProjectMember(ctx, &pb.DeleteProjectMember_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		MemberId:  input.GetMemberId(),
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}

	return httputil.StatusOK(c, "Member deleted", nil)
}

// @Summary      List users without project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true "Project ID"
// @Param        owner_id        path     uuid true "Owner ID"
// @Param        name            path     string true "Name"
// @Success      200             {object} httputil.HTTPResponse
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members/search [get]
func (h *handler) getUsersWithoutProject(c *fiber.Ctx) error {
	input := new(pb.ActivityRequest)

	if err := c.QueryParser(input); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	userParameter := middleware.AuthUser(c)
	ownerID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

	members, err := rClient.UsersWithoutProject(ctx, &pb.UsersWithoutProject_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		Name:      input.GetName(),
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}

	return httputil.StatusOK(c, "Users without project", members)
}

// @Summary      Update member status of project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     pb.UpdateProjectMemberStatus_Request{}
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/members/active [patch]
func (h *handler) patchProjectMemberStatus(c *fiber.Ctx) error {
	input := new(pb.UpdateProjectMemberStatus_Request)

	if err := c.BodyParser(input); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	userParameter := middleware.AuthUser(c)
	ownerID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

	_, err := rClient.UpdateProjectMemberStatus(ctx, &pb.UpdateProjectMemberStatus_Request{
		OwnerId:   ownerID,
		MemberId:  input.GetMemberId(),
		ProjectId: input.GetProjectId(),
		Status:    input.GetStatus(),
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}

	// message section
	message := "Member is online"
	if !input.GetStatus() {
		message = "Member is offline"
	}

	return httputil.StatusOK(c, message, nil)
}

// @Summary      Show invites on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        owner_id        path     uuid false "Project owner ID"
// @Param        project_id      path     uuid true  "Project ID"
// @Param        member_id       path     uuid true  "Member ID"
// @Success      200             {object} httputil.HTTPResponse{data=pb.ListProjectMembersInvite_Response}
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members/invite [get]
func (h *handler) getProjectMembersInvite(c *fiber.Ctx) error {
	input := new(pb.ProjectMember_Request)

	if err := c.QueryParser(input); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	userParameter := middleware.AuthUser(c)
	ownerID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

	pagination := httputil.GetPaginationFromCtx(c)
	members, err := rClient.ListProjectMembersInvite(ctx, &pb.ListProjectMembersInvite_Request{
		Limit:     pagination.GetLimit(),
		Offset:    pagination.GetOffset(),
		SortBy:    "created:ASC",
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}

	return httputil.StatusOK(c, "Member invites", members)
}

// @Summary      Invite a new member on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     pb.AddProjectMemberInvite_Request{}
// @Success      200         {object} httputil.HTTPResponse{data=pb.AddProjectMemberInvite_Response}
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/members/invite [post]
func (h *handler) addProjectMemberInvite(c *fiber.Ctx) error {
	input := new(pb.AddProjectMemberInvite_Request)

	if err := c.BodyParser(input); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

	member, err := rClient.AddProjectMemberInvite(ctx, &pb.AddProjectMemberInvite_Request{
		OwnerId:     userID,
		ProjectId:   input.GetProjectId(),
		UserName:    input.GetUserName(),
		UserSurname: input.GetUserSurname(),
		Email:       input.GetEmail(),
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}

	mailData := map[string]string{
		"Link": fmt.Sprintf("%s/invite/project/%s", internal.GetString("APP_DSN", "https://app.werbot.com"), member.GetInvite()),
	}
	go mail.Send(input.GetEmail(), "Invitation to the project", "project-invite", mailData)

	return httputil.StatusOK(c, "Member invited", member)
}

// @Summary      Delete invite on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true "Project ID"
// @Param        owner_id        path     uuid true "Owner ID"
// @Param        invite_id       path     uuid true "Invite ID"
// @Success      200             {object} httputil.HTTPResponse
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members/invite [delete]
func (h *handler) deleteProjectMemberInvite(c *fiber.Ctx) error {
	input := new(pb.DeleteProjectMemberInvite_Request)

	if err := c.QueryParser(input); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	userParameter := middleware.AuthUser(c)
	ownerID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

	_, err := rClient.DeleteProjectMemberInvite(ctx, &pb.DeleteProjectMemberInvite_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		InviteId:  input.GetInviteId(),
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}

	return httputil.StatusOK(c, "Invite deleted", nil)
}

// @Summary      Confirmation of the invitation to join the project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        invite      path     uuid true "Invite"
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,500     {object} httputil.HTTPResponse
// @Router       /v1/members/invite/:invite [post]
func (h *handler) postProjectMembersInviteActivate(c *fiber.Ctx) error {
	request := new(pb.ProjectMemberInviteActivate_Request)
	request.Invite = c.Params("invite")

	if err := validate.Struct(request); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, err)
	}

	userParameter := middleware.AuthUser(c)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

	project, err := rClient.ProjectMemberInviteActivate(ctx, &pb.ProjectMemberInviteActivate_Request{
		Invite: request.GetInvite(),
		UserId: userParameter.User.GetUserId(),
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}

	return httputil.StatusOK(c, "Invitation confirmed", project)
}
