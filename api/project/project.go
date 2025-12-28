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

// @Summary Get Projects
// @Description Retrieve a list of projects with pagination and sorting.
// @Tags projects
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Param sort_by query string false "Sort by for pagination"
// @Success 200 {object} webutil.HTTPResponse{result=projectmessage.Projects_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/projects [get]
func (h *Handler) projects(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	pagination := webutil.GetPaginationFromCtx(c)
	request := &projectmessage.Projects_Request{
		IsAdmin: sessionData.IsProfileAdmin(),
		OwnerId: sessionData.ProfileID(c.Query("owner_id")),
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		SortBy:  "id:ASC",
	}

	rClient := projectrpc.NewProjectHandlersClient(h.Grpc)
	projects, err := rClient.Projects(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(projects)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Projects", result)
}

// @Summary Get Project
// @Description Retrieve detailed information about a specific project.
// @Tags projects
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param member_id path string true "Member UUID"
// @Success 200 {object} webutil.HTTPResponse{result=projectmessage.Project_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/projects/{project_id} [get]
func (h *Handler) project(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &projectmessage.Project_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
	}

	rClient := projectrpc.NewProjectHandlersClient(h.Grpc)
	project, err := rClient.Project(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(project)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Project", result)
}

// @Summary Add a new project
// @Description Adds a new project for the authenticated profile
// @Tags projects
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Success 200 {object} webutil.HTTPResponse{result=projectmessage.AddProject_Request}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/projects [post]
func (h *Handler) addProject(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &projectmessage.AddProject_Request{
		OwnerId: sessionData.ProfileID(c.Query("owner_id")),
	}

	_ = webutil.Parse(c, request).Body()

	rClient := projectrpc.NewProjectHandlersClient(h.Grpc)
	project, err := rClient.AddProject(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(project)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Project(request.GetOwnerId(), event.ProjectProject, event.OnCreate, request)

	return webutil.StatusOK(c, "Project added", result)
}

// @Summary Update a project
// @Description Updates the details of an existing project based on the provided project UUID and owner UUID.
// @Tags projects
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param body body projectmessage.UpdateProject_Request true "Update Project Request Body"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router //v1/projects/{project_id} [put]
func (h *Handler) updateProject(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &projectmessage.UpdateProject_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
	}

	_ = webutil.Parse(c, request).Body(true)

	rClient := projectrpc.NewProjectHandlersClient(h.Grpc)
	if _, err := rClient.UpdateProject(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Project(request.GetOwnerId(), event.ProjectSetting, event.OnUpdate, request)

	return webutil.StatusOK(c, "Project updated", nil)
}

// @Summary Delete a project
// @Description Deletes an existing project based on the provided project UUID and owner UUID.
// @Tags projects
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/projects/{project_id} [delete]
func (h *Handler) deleteProject(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &projectmessage.DeleteProject_Request{
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
	}

	rClient := projectrpc.NewProjectHandlersClient(h.Grpc)
	if _, err := rClient.DeleteProject(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Project(request.GetOwnerId(), event.ProjectProject, event.OnRemove, request)

	return webutil.StatusOK(c, "Project deleted", nil)
}
