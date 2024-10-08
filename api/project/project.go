package project

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/event"
	projectpb "github.com/werbot/werbot/internal/grpc/project/proto/project"
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
// @Success 200 {object} webutil.HTTPResponse{result=projectpb.Projects_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/projects [get]
func (h *Handler) projects(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	pagination := webutil.GetPaginationFromCtx(c)
	request := &projectpb.Projects_Request{
		IsAdmin: sessionData.IsUserAdmin(),
		OwnerId: sessionData.UserID(c.Query("owner_id")),
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		SortBy:  "id:ASC",
	}

	rClient := projectpb.NewProjectHandlersClient(h.Grpc)
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
// @Success 200 {object} webutil.HTTPResponse{result=projectpb.Project_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/projects/{project_id} [get]
func (h *Handler) project(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &projectpb.Project_Request{
		IsAdmin:   sessionData.IsUserAdmin(),
		OwnerId:   sessionData.UserID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
	}

	rClient := projectpb.NewProjectHandlersClient(h.Grpc)
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
// @Description Adds a new project for the authenticated user
// @Tags projects
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Success 200 {object} webutil.HTTPResponse{result=projectpb.AddProject_Request}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/projects [post]
func (h *Handler) addProject(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &projectpb.AddProject_Request{
		OwnerId: sessionData.UserID(c.Query("owner_id")),
	}

	_ = webutil.Parse(c, request).Body()

	rClient := projectpb.NewProjectHandlersClient(h.Grpc)
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
// @Param body body projectpb.UpdateProject_Request true "Update Project Request Body"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router //v1/projects/{project_id} [put]
func (h *Handler) updateProject(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &projectpb.UpdateProject_Request{
		IsAdmin:   sessionData.IsUserAdmin(),
		OwnerId:   sessionData.UserID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
	}

	_ = webutil.Parse(c, request).Body(true)

	rClient := projectpb.NewProjectHandlersClient(h.Grpc)
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
	sessionData := session.AuthUser(c)
	request := &projectpb.DeleteProject_Request{
		OwnerId:   sessionData.UserID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
	}

	rClient := projectpb.NewProjectHandlersClient(h.Grpc)
	if _, err := rClient.DeleteProject(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Project(request.GetOwnerId(), event.ProjectProject, event.OnRemove, request)

	return webutil.StatusOK(c, "Project deleted", nil)
}
