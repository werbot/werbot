package project

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"

	pb "github.com/werbot/werbot/api/proto/project"
)

// @Summary      Show information about project or list of all projects
// @Tags         projects
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true  "Project ID"
// @Param        owner_id        path     uuid false "Project owner ID. Parameter Accessible with ROLE_ADMIN rights"
// @Success      200             {object} webutil.HTTPResponse{data=pb.ListProjects_Response}
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/projects [get]
func (h *handler) getProject(c *fiber.Ctx) error {
	request := new(pb.Project_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.Project_RequestMultiError) {
			e := err.(pb.Project_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewProjectHandlersClient(h.Grpc.Client)

	// show all projects
	if request.GetProjectId() == "" {
		pagination := webutil.GetPaginationFromCtx(c)
		sanitizeSQL, _ := sanitize.SQL(`"project"."owner_id" = $1`, userID)
		projects, err := rClient.ListProjects(ctx, &pb.ListProjects_Request{
			Limit:  pagination.GetLimit(),
			Offset: pagination.GetOffset(),
			SortBy: "id:ASC",
			Query:  sanitizeSQL,
		})
		if err != nil {
			return webutil.FromGRPC(c, h.log, err)
		}
		if projects.GetTotal() == 0 {
			return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
		}

		return webutil.StatusOK(c, msgProjects, projects)
	}

	// show project information
	project, err := rClient.Project(ctx, &pb.Project_Request{
		OwnerId:   userID,
		ProjectId: request.GetProjectId(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}
	// if project == nil {
	// 	return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	// }

	// If RoleUser_ADMIN - show detailed information
	if userParameter.IsUserAdmin() {
		return webutil.StatusOK(c, msgProjectInfo, project)
	}

	return webutil.StatusOK(c, msgProjectInfo, &pb.Project_Response{
		Title: project.GetTitle(),
		Login: project.GetLogin(),
	})
}

// @Summary      Adding a new project
// @Tags         projects
// @Accept       json
// @Produce      json
// @Param        req         body     pb.AddProject_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=pb.AddProject_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/projects [post]
func (h *handler) addProject(c *fiber.Ctx) error {
	request := new(pb.AddProject_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.AddProject_RequestMultiError) {
			e := err.(pb.AddProject_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewProjectHandlersClient(h.Grpc.Client)

	project, err := rClient.AddProject(ctx, &pb.AddProject_Request{
		OwnerId: userID,
		Login:   request.GetLogin(),
		Title:   request.GetTitle(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgProjectAdded, project)
}

// @Summary      Updating project information.
// @Tags         projects
// @Accept       json
// @Produce      json
// @Param        req             body     pb.UpdateProject_Request{}
// @Success      200             {object} webutil.HTTPResponse{data=pb.UpdateProject_Response}
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/projects [patch]
func (h *handler) patchProject(c *fiber.Ctx) error {
	request := new(pb.UpdateProject_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.UpdateProject_RequestMultiError) {
			e := err.(pb.UpdateProject_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewProjectHandlersClient(h.Grpc.Client)

	_, err := rClient.UpdateProject(ctx, &pb.UpdateProject_Request{
		OwnerId:   userID,
		ProjectId: request.GetProjectId(),
		Title:     request.GetTitle(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgProjectUpdated, nil)
}

// @Summary      Delete project
// @Tags         projects
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true "Project ID"
// @Param        owner_id        path     uuid true "Owner ID"
// @Success      200             {object} webutil.HTTPResponse
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/projects [delete]
func (h *handler) deleteProject(c *fiber.Ctx) error {
	request := new(pb.DeleteProject_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.DeleteProject_RequestMultiError) {
			e := err.(pb.DeleteProject_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewProjectHandlersClient(h.Grpc.Client)

	_, err := rClient.DeleteProject(ctx, &pb.DeleteProject_Request{
		ProjectId: request.GetProjectId(),
		OwnerId:   userID,
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgProjectDeleted, nil)
}
