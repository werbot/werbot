package token

/*
import (
	"github.com/gofiber/fiber/v2"

	event "github.com/werbot/werbot/internal/core/event/recorder"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

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
// @Success 200 {object} webutil.HTTPResponse{result=tokenpb.MembersToken_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/invite/project/{project_id} [get]
func (h *Handler) projectMembersInvite(c *fiber.Ctx) error {
	pagination := webutil.GetPaginationFromCtx(c)
	sessionData := session.AuthProfile(c)
	request := &tokenpb.MembersToken_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		Limit:     pagination.Limit,
		Offset:    pagination.Offset,
		SortBy:    `"project_invite"."status":DESC,"project_invite"."updated_at":DESC`,
	}

	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
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
// @Param body body tokenpb.AddMemberToken_Request true "Request body for adding a member invite"
// @Success 200 {object} webutil.HTTPResponse{result=tokenpb.AddMemberToken_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/invite/project/{project_id} [post]
func (h *Handler) addProjectMemberInvite(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &tokenpb.AddMemberToken_Request{
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
	}

	_ = webutil.Parse(c, request).Body()

	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
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
	request := &tokenpb.DeleteMemberToken_Request{
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		Token:     c.Params("token"),
	}

	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
	if _, err := rClient.DeleteMemberInvite(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Project(request.GetOwnerId(), event.ProjectMember, event.OnRemove, request)

	return webutil.StatusOK(c, "Invite deleted", nil)
}
*/
