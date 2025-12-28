package member

import (
	"github.com/gofiber/fiber/v2"

	event "github.com/werbot/werbot/internal/core/event/recorder"
	memberrpc "github.com/werbot/werbot/internal/core/member/proto/rpc"
	membermessage "github.com/werbot/werbot/internal/core/member/proto/message"
	profileenum "github.com/werbot/werbot/internal/core/profile/proto/enum"
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
// @Success 200 {object} webutil.HTTPResponse{result1=membermessage.ProjectMembers_Response,result2=membermessage.AddProjectMember_Response}
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

		request := &membermessage.ProfilesWithoutProject_Request{
			ProjectId: c.Params("project_id"),
			Limit:     pagination.Limit,
			Offset:    pagination.Offset,
			SortBy:    `"profile"."alias":ASC`,
		}

		_ = webutil.Parse(c, request).Query()

		rClient := memberrpc.NewMemberHandlersClient(h.Grpc)
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
	request := &membermessage.ProjectMembers_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		Limit:     pagination.Limit,
		Offset:    pagination.Offset,
		SortBy:    `"project_member"."created_at":DESC`,
	}

	_ = webutil.Parse(c, request).Query()

	rClient := memberrpc.NewMemberHandlersClient(h.Grpc)
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
// @Success 200 {object} webutil.HTTPResponse{result=membermessage.ProjectMember_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/project/{project_id}/{member_id} [get]
func (h *Handler) projectMember(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &membermessage.ProjectMember_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		MemberId:  c.Params("member_id"),
	}

	rClient := memberrpc.NewMemberHandlersClient(h.Grpc)
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
// @Param body body membermessage.AddProjectMember_Request true "Adds a new member Request Body"
// @Success 200 {object} webutil.HTTPResponse{result=membermessage.AddProjectMember_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/project/{project_id} [post]
func (h *Handler) addProjectMember(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &membermessage.AddProjectMember_Request{
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		Role:      profileenum.Role_user,
	}

	_ = webutil.Parse(c, request).Body()

	rClient := memberrpc.NewMemberHandlersClient(h.Grpc)
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
// @Param body body membermessage.UpdateProjectMember_Request true "Updates the role or status Request Body"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/project/{project_id}/{member_id} [put]
func (h *Handler) updateProjectMember(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &membermessage.UpdateProjectMember_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		MemberId:  c.Params("member_id"),
	}

	_ = webutil.Parse(c, request).Body(true)

	rClient := memberrpc.NewMemberHandlersClient(h.Grpc)
	if _, err := rClient.UpdateProjectMember(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	var eventType event.Type
	switch request.GetSetting().(type) {
	case *membermessage.UpdateProjectMember_Request_Role:
		eventType = event.OnUpdate
	case *membermessage.UpdateProjectMember_Request_Active:
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
	request := &membermessage.DeleteProjectMember_Request{
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		MemberId:  c.Params("member_id"),
	}

	rClient := memberrpc.NewMemberHandlersClient(h.Grpc)
	if _, err := rClient.DeleteProjectMember(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Project(request.GetOwnerId(), event.ProjectMember, event.OnRemove, request)

	return webutil.StatusOK(c, "Member deleted", nil)
}
