package member

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/mail"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"

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
// @Success      200             {object} webutil.HTTPResponse{data=pb.ProjectMember_Response}
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/members [get]
func (h *Handler) getProjectMember(c *fiber.Ctx) error {
	request := new(pb.ProjectMember_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.ProjectMember_RequestMultiError) {
			e := err.(pb.ProjectMember_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

	// show all members
	if request.GetMemberId() == "" {
		pagination := webutil.GetPaginationFromCtx(c)
		sanitizeSQL, _ := sanitize.SQL(`owner_id = $1 AND project_id = $2`, request.GetOwnerId(), request.GetProjectId())
		members, err := rClient.ListProjectMembers(ctx, &pb.ListProjectMembers_Request{
			Limit:  pagination.GetLimit(),
			Offset: pagination.GetOffset(),
			SortBy: "member_created:DESC",
			Query:  sanitizeSQL,
		})
		if err != nil {
			return webutil.FromGRPC(c, h.log, err)
		}
		if members.GetTotal() == 0 {
			return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
		}

		return webutil.StatusOK(c, msgMembers, members)
	}

	// show information about the member
	member, err := rClient.ProjectMember(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}
	// if member == nil {
	// 	return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	// }

	return webutil.StatusOK(c, msgMemberInfo, member)
}

// @Summary      Adding a new member on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     pb.AddProject_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=pb.AddProjectMember_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/members [post]
func (h *Handler) addProjectMember(c *fiber.Ctx) error {
	request := new(pb.AddProjectMember_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.AddProjectMember_RequestMultiError) {
			e := err.(pb.AddProjectMember_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())
	request.Role = pb_user.RoleUser_USER // TODO directly install the role of the user

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)
	member, err := rClient.AddProjectMember(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgMemberAdded, member)
}

// @Summary      Updating member information on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req             body     pb.UpdateMember_Request{}
// @Success      200             {object} webutil.HTTPResponse{data=pb.UpdateProjectMember_Response}
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/members [patch]
func (h *Handler) patchProjectMember(c *fiber.Ctx) error {
	request := new(pb.UpdateProjectMember_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.UpdateProjectMember_RequestMultiError) {
			e := err.(pb.UpdateProjectMember_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)
	_, err := rClient.UpdateProjectMember(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgMemberUpdated, nil)
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
	request := new(pb.DeleteProjectMember_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.DeleteProjectMember_RequestMultiError) {
			e := err.(pb.DeleteProjectMember_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)
	_, err := rClient.DeleteProjectMember(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgMemberDeleted, nil)
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
	request := new(pb.UsersWithoutProject_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.UsersWithoutProject_RequestMultiError) {
			e := err.(pb.UsersWithoutProject_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)
	members, err := rClient.UsersWithoutProject(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgUsersWithoutProject, members)
}

// @Summary      Update member status of project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     pb.UpdateProjectMemberStatus_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/members/active [patch]
func (h *Handler) patchProjectMemberStatus(c *fiber.Ctx) error {
	request := new(pb.UpdateProjectMemberStatus_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.UpdateProjectMemberStatus_RequestMultiError) {
			e := err.(pb.UpdateProjectMemberStatus_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)
	_, err := rClient.UpdateProjectMemberStatus(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	// message section
	message := msgMemberIsOnline
	if !request.GetStatus() {
		message = msgMemberIsOffline
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
// @Success      200             {object} webutil.HTTPResponse{data=pb.ListProjectMembersInvite_Response}
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/members/invite [get]
func (h *Handler) getProjectMembersInvite(c *fiber.Ctx) error {
	request := new(pb.ListProjectMembersInvite_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.ListProjectMembersInvite_RequestMultiError) {
			e := err.(pb.ListProjectMembersInvite_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	pagination := webutil.GetPaginationFromCtx(c)
	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())
	request.SortBy = `"created":ASC`
	request.Limit = pagination.GetLimit()
	request.Offset = pagination.GetOffset()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)
	members, err := rClient.ListProjectMembersInvite(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgMemberInvites, members)
}

// @Summary      Invite a new member on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     pb.AddProjectMemberInvite_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=pb.AddProjectMemberInvite_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/members/invite [post]
func (h *Handler) addProjectMemberInvite(c *fiber.Ctx) error {
	request := new(pb.AddProjectMemberInvite_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.AddProjectMemberInvite_RequestMultiError) {
			e := err.(pb.AddProjectMemberInvite_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)
	member, err := rClient.AddProjectMemberInvite(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	mailData := map[string]string{
		"Link": fmt.Sprintf("%s/invite/project/%s", internal.GetString("APP_DSN", "https://app.werbot.com"), member.GetInvite()),
	}
	go mail.Send(request.GetEmail(), msgProjectInvitation, "project-invite", mailData)

	return webutil.StatusOK(c, msgMemberInvited, member)
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
	request := new(pb.DeleteProjectMemberInvite_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.DeleteProjectMemberInvite_RequestMultiError) {
			e := err.(pb.DeleteProjectMemberInvite_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)
	_, err := rClient.DeleteProjectMemberInvite(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgInviteDeleted, nil)
}

// @Summary      Confirmation of the invitation to join the project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        invite      path     uuid true "Invite"
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,500     {object} webutil.HTTPResponse
// @Router       /v1/members/invite/:invite [post]
func (h *Handler) postProjectMembersInviteActivate(c *fiber.Ctx) error {
	request := new(pb.ProjectMemberInviteActivate_Request)
	request.Invite = c.Params("invite")

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.ProjectMemberInviteActivate_RequestMultiError) {
			e := err.(pb.ProjectMemberInviteActivate_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.User.GetUserId()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)
	project, err := rClient.ProjectMemberInviteActivate(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgInviteConfirmed, project)
}
