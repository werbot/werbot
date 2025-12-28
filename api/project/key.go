package project

import (
	"github.com/gofiber/fiber/v2"

	event "github.com/werbot/werbot/internal/core/event/recorder"
	projectrpc "github.com/werbot/werbot/internal/core/project/proto/rpc"
	projectmessage "github.com/werbot/werbot/internal/core/project/proto/message"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Get project keys
// @Description Retrieves keys associated with a specific project based on the provided project UUID and owner UUID.
// @Tags projects
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Param sort_by query string false "Sort by for pagination"
// @Success 200 {object} webutil.HTTPResponse{result=projectmessage.ProjectKeys_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/projects/{project_id}/keys [get]
func (h *Handler) projectKeys(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	pagination := webutil.GetPaginationFromCtx(c)
	request := &projectmessage.ProjectKeys_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		Limit:     pagination.Limit,
		Offset:    pagination.Offset,
		SortBy:    "id:ASC",
	}

	rClient := projectrpc.NewProjectHandlersClient(h.Grpc)
	projects, err := rClient.ProjectKeys(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(projects)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Project keys", result)
}

// @Summary Get project key
// @Description Retrieves detailed information about a specific key associated with a project based on the provided project UUID and key UUID.
// @Tags projects
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param key_id path string true "Key UUID"
// @Success 200 {object} webutil.HTTPResponse{result=projectmessage.ProjectKey_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/projects/{project_id}/keys/{key_id} [get]
func (h *Handler) projectKey(c *fiber.Ctx) error {
	request := &projectmessage.ProjectKey_Request{}

	key := c.Params("key")
	if key != "" { // without authorizations, using in agent
		request.Type = &projectmessage.ProjectKey_Request_Public{
			Public: &projectmessage.ProjectKey_Public{
				Key: key,
			},
		}
	} else {
		sessionData := session.AuthProfile(c)
		request.Type = &projectmessage.ProjectKey_Request_Private{
			Private: &projectmessage.ProjectKey_Private{
				IsAdmin:   sessionData.IsProfileAdmin(),
				OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
				ProjectId: c.Params("project_id"),
				KeyId:     c.Params("key_id"),
			},
		}
	}

	rClient := projectrpc.NewProjectHandlersClient(h.Grpc)
	project, err := rClient.ProjectKey(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(project)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Project key", result)
}

// @Summary Add project key
// @Description Adds a new key to a specified project based on the provided project UUID and owner UUID.
// @Tags projects
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Success 200 {object} webutil.HTTPResponse{result=projectmessage.AddProjectKey_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/projects/{project_id}/keys [post]
func (h *Handler) addProjectKey(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &projectmessage.AddProjectKey_Request{
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
	}

	_ = webutil.Parse(c, request).Body()

	rClient := projectrpc.NewProjectHandlersClient(h.Grpc)
	project, err := rClient.AddProjectKey(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(project)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Project(request.GetOwnerId(), event.ProjectSetting, event.OnCreate, request)

	return webutil.StatusOK(c, "Project added", result)
}

// @Summary Delete project key
// @Description Deletes an existing key from a specified project based on the provided project UUID, owner UUID, and key UUID.
// @Tags projects
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param key_id path string true "Key UUID"
// @Param project_id path string true "Project UUID"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/projects/{project_id}/keys/{key_id} [delete]
func (h *Handler) deleteProjectKey(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &projectmessage.DeleteProjectKey_Request{
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		KeyId:     c.Params("key_id"),
	}

	rClient := projectrpc.NewProjectHandlersClient(h.Grpc)
	if _, err := rClient.DeleteProjectKey(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Project(request.GetOwnerId(), event.ProjectSetting, event.OnRemove, request)

	return webutil.StatusOK(c, "Project key deleted", nil)
}
