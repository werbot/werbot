package project

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal/grpc"
	projectpb "github.com/werbot/werbot/internal/grpc/project/proto"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"
)

// @Summary      Show information about project or list of all projects
// @Tags         projects
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true  "Project ID"
// @Param        owner_id        path     uuid false "Project owner ID. Parameter Accessible with ROLE_ADMIN rights"
// @Success      200             {object} webutil.HTTPResponse{data=projectpb.ListProjects_Response}
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/projects [get]
func (h *Handler) getProject(c *fiber.Ctx) error {
	request := new(projectpb.Project_Request)

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

	rClient := projectpb.NewProjectHandlersClient(h.Grpc.Client)

	// show all projects
	if request.GetProjectId() == "" {
		pagination := webutil.GetPaginationFromCtx(c)
		sanitizeSQL, _ := sanitize.SQL(`"project"."owner_id" = $1`,
			request.GetOwnerId(),
		)
		projects, err := rClient.ListProjects(ctx, &projectpb.ListProjects_Request{
			Limit:  pagination.GetLimit(),
			Offset: pagination.GetOffset(),
			SortBy: "id:ASC",
			Query:  sanitizeSQL,
		})
		if err != nil {
			return webutil.FromGRPC(c, err)
		}
		if projects.GetTotal() == 0 {
			return webutil.FromGRPC(c, status.Error(codes.NotFound, "Not found"))
		}

		return webutil.StatusOK(c, "projects", projects)
	}

	// show project information
	project, err := rClient.Project(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}
	// if project == nil {
	// 	return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	// }

	// If Role_admin - show detailed information
	if userParameter.IsUserAdmin() {
		return webutil.StatusOK(c, "project information", project)
	}

	return webutil.StatusOK(c, "project information", &projectpb.Project_Response{
		Title: project.GetTitle(),
		Login: project.GetLogin(),
	})
}

// @Summary      Adding a new project
// @Tags         projects
// @Accept       json
// @Produce      json
// @Param        req         body     projectpb.AddProject_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=projectpb.AddProject_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/projects [post]
func (h *Handler) addProject(c *fiber.Ctx) error {
	request := new(projectpb.AddProject_Request)

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

	rClient := projectpb.NewProjectHandlersClient(h.Grpc.Client)
	project, err := rClient.AddProject(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "project added", project)
}

// @Summary      Updating project information.
// @Tags         projects
// @Accept       json
// @Produce      json
// @Param        req             body     projectpb.UpdateProject_Request{}
// @Success      200             {object} webutil.HTTPResponse{data=projectpb.UpdateProject_Response}
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/projects [patch]
func (h *Handler) updateProject(c *fiber.Ctx) error {
	request := new(projectpb.UpdateProject_Request)

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

	rClient := projectpb.NewProjectHandlersClient(h.Grpc.Client)
	if _, err := rClient.UpdateProject(ctx, request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "project updated", nil)
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
func (h *Handler) deleteProject(c *fiber.Ctx) error {
	request := new(projectpb.DeleteProject_Request)

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

	rClient := projectpb.NewProjectHandlersClient(h.Grpc.Client)
	if _, err := rClient.DeleteProject(ctx, request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "project deleted", nil)
}
