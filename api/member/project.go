package member

import (
	"github.com/gofiber/fiber/v2"

	memberpb "github.com/werbot/werbot/internal/core/member/proto/member"
	profilepb "github.com/werbot/werbot/internal/core/profile/proto/profile"
	"github.com/werbot/werbot/internal/event"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Retrieve project members
// @Description Get a list of members for a given project UUID with pagination and sorting options
// @Tags members
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param addon path string false "Search profiles without project for admin"
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Param sort_by query string false "Sort by for pagination"
// @Success 200 {object} webutil.HTTPResponse{result1=memberpb.ProjectMembers_Response,result2=memberpb.AddProjectMember_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/project/{project_id} [get]
func (h *Handler) projectMembers(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	pagination := webutil.GetPaginationFromCtx(c)

	// search profiles without project, access only for admin
	if c.Params("addon") == "search" {
		if !sessionData.IsProfileAdmin() {
			return webutil.StatusNotFound(c, nil)
		}

		request := &memberpb.ProfilesWithoutProject_Request{
			ProjectId: c.Params("project_id"),
			Limit:     pagination.Limit,
			Offset:    pagination.Offset,
			SortBy:    `"profile"."alias":ASC`,
		}

		_ = webutil.Parse(c, request).Query()

		rClient := memberpb.NewMemberHandlersClient(h.Grpc)
		members, err := rClient.ProfilesWithoutProject(c.UserContext(), request)
		if err != nil {
			return webutil.FromGRPC(c, err)
		}

		result, err := protoutils.ConvertProtoMessageToMap(members)
		if err != nil {
			return webutil.FromGRPC(c, err)
		}

		return webutil.StatusOK(c, "Profiles without project", result)
	}

	// default show
	request := &memberpb.ProjectMembers_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		Limit:     pagination.Limit,
		Offset:    pagination.Offset,
		SortBy:    `"project_member"."created_at":DESC`,
	}

	_ = webutil.Parse(c, request).Query()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	members, err := rClient.ProjectMembers(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(members)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Members", result)
}

// @Summary Retrieve project member information
// @Description Get details of a specific member within a given project by project UUID and member UUID
// @Tags members
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param member_id path string true "Member UUID"
// @Success 200 {object} webutil.HTTPResponse{result=memberpb.ProjectMember_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/project/{project_id}/{member_id} [get]
func (h *Handler) projectMember(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &memberpb.ProjectMember_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		MemberId:  c.Params("member_id"),
	}

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	member, err := rClient.ProjectMember(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(member)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Member", result)
}

// @Summary Add Project Member, access only for admin
// @Description Adds a new member to the specified project with a given role
// @Tags members
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "UUID of the project"
// @Param body body memberpb.AddProjectMember_Request true "Adds a new member Request Body"
// @Success 200 {object} webutil.HTTPResponse{result=memberpb.AddProjectMember_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/project/{project_id} [post]
func (h *Handler) addProjectMember(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &memberpb.AddProjectMember_Request{
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		Role:      profilepb.Role_user,
	}

	_ = webutil.Parse(c, request).Body()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	member, err := rClient.AddProjectMember(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(member)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Project(request.GetOwnerId(), event.ProjectMember, event.OnCreate, request)

	return webutil.StatusOK(c, "Member added", result)
}

// @Summary Update Project Member
// @Description Updates the role or status of an existing project member
// @Tags members
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "UUID of the project"
// @Param member_id path string true "UUID of the member to be updated"
// @Param body body memberpb.UpdateProjectMember_Request true "Updates the role or status Request Body"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/project/{project_id}/{member_id} [put]
func (h *Handler) updateProjectMember(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &memberpb.UpdateProjectMember_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		MemberId:  c.Params("member_id"),
	}

	_ = webutil.Parse(c, request).Body(true)

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	if _, err := rClient.UpdateProjectMember(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	var eventType event.EventType
	switch request.GetSetting().(type) {
	case *memberpb.UpdateProjectMember_Request_Role:
		eventType = event.OnUpdate
	case *memberpb.UpdateProjectMember_Request_Active:
		if request.GetActive() {
			eventType = event.OnOffline
		} else {
			eventType = event.OnOnline
		}
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Project(request.GetOwnerId(), event.ProjectMember, eventType, request)

	return webutil.StatusOK(c, "Member updated", nil)
}

// @Summary Delete Project Member
// @Description Deletes an existing project member
// @Tags members
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "UUID of the project"
// @Param member_id path string true "UUID of the member to be deleted"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/project/{project_id}/{member_id} [delete]
func (h *Handler) deleteProjectMember(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &memberpb.DeleteProjectMember_Request{
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		MemberId:  c.Params("member_id"),
	}

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	if _, err := rClient.DeleteProjectMember(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Project(request.GetOwnerId(), event.ProjectMember, event.OnRemove, request)

	return webutil.StatusOK(c, "Member deleted", nil)
}

// @Summary Invite members to a project
// @Description This endpoint allows an authenticated profile to invite members to a specified project
// @Tags members
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Param sort_by query string false "Sort by for pagination"
// @Success 200 {object} webutil.HTTPResponse{result=memberpb.MembersInvite_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/invite/project/{project_id} [get]
func (h *Handler) projectMembersInvite(c *fiber.Ctx) error {
	pagination := webutil.GetPaginationFromCtx(c)
	sessionData := session.AuthProfile(c)
	request := &memberpb.MembersInvite_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		Limit:     pagination.Limit,
		Offset:    pagination.Offset,
		SortBy:    `"project_invite"."status":DESC,"project_invite"."updated_at":DESC`,
	}

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	members, err := rClient.MembersInvite(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(members)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Member invites", result)
}

// @Summary Invite a new member to a project
// @Description This endpoint allows an authenticated profile to invite a new member to a specific project
// @Tags members
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param body body memberpb.AddMemberInvite_Request true "Request body for adding a member invite"
// @Success 200 {object} webutil.HTTPResponse{result=memberpb.AddMemberInvite_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/invite/project/{project_id} [post]
func (h *Handler) addProjectMemberInvite(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &memberpb.AddMemberInvite_Request{
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
	}

	_ = webutil.Parse(c, request).Body()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	member, err := rClient.AddMemberInvite(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(member)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Project(request.GetOwnerId(), event.ProjectMember, event.OnMessage, request)

	return webutil.StatusOK(c, "Member invited", result)
}

// @Summary Delete Project Member Invite
// @Description Deletes an invite for a project member based on the provided profile, project, and token UUIDs
// @Tags members
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param invite path string true "Invite UUID"
// @Success 200 {object} webutil.HTTPResponse{}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/invite/project/{project_id}/{token} [delete]
func (h *Handler) deleteProjectMemberInvite(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &memberpb.DeleteMemberInvite_Request{
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		Token:     c.Params("token"),
	}

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	if _, err := rClient.DeleteMemberInvite(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Project(request.GetOwnerId(), event.ProjectMember, event.OnRemove, request)

	return webutil.StatusOK(c, "Invite deleted", nil)
}

// TODO
// @Summary      Confirmation of the invitation to join the project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        invite      path     uuid true "Invite"
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,500     {object} webutil.HTTPResponse
// @Router       /v1/members/invite/:token [get]
func (h *Handler) membersInviteActivate(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &memberpb.MemberInviteActivate_Request{
		ProfileId: sessionData.ProfileID(c.Query("owner_id")),
		Token:     c.Params("token"),
	}

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	project, err := rClient.MemberInviteActivate(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(project)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// TODO depending on the response, redirect to the registration page, authorization or display a message about successful activation

	return webutil.StatusOK(c, "Invite confirmed", result)
}
