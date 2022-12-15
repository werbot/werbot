package project

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/utils/validate"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/api/proto/project"
)

// @Summary      Show information about project or list of all projects
// @Tags         projects
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true  "Project ID"
// @Param        owner_id        path     uuid false "Project owner ID. Parameter Accessible with ROLE_ADMIN rights"
// @Success      200             {object} httputil.HTTPResponse{data=pb.ListProjects_Response}
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/projects [get]
func (h *Handler) getProject(c *fiber.Ctx) error {
	input := new(pb.GetProject_Request)
	c.QueryParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewProjectHandlersClient(h.grpc.Client)

	// show all projects
	if input.GetProjectId() == "" {
		pagination := httputil.GetPaginationFromCtx(c)
		sanitizeSQL, _ := sanitize.SQL(`"project"."owner_id" = $1`, userID)
		projects, err := rClient.ListProjects(ctx, &pb.ListProjects_Request{
			Limit:  pagination.GetLimit(),
			Offset: pagination.GetOffset(),
			SortBy: "id:ASC",
			Query:  sanitizeSQL,
		})
		if err != nil {
			return httputil.ReturnGRPCError(c, err)
		}
		if projects.GetTotal() == 0 {
			return httputil.StatusNotFound(c, internal.ErrNotFound, nil)
		}
		return httputil.StatusOK(c, "Projects", projects)
	}

	// show project information
	project, err := rClient.GetProject(ctx, &pb.GetProject_Request{
		OwnerId:   userID,
		ProjectId: input.GetProjectId(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	if project == nil {
		return httputil.StatusNotFound(c, internal.ErrNotFound, nil)
	}

	// If RoleUser_ADMIN - show detailed information
	if userParameter.IsUserAdmin() {
		return httputil.StatusOK(c, "Project information", project)
	}

	return httputil.StatusOK(c, "Project information", &pb.GetProject_Response{
		Title: project.GetTitle(),
		Login: project.GetLogin(),
	})
}

// @Summary      Adding a new project
// @Tags         projects
// @Accept       json
// @Produce      json
// @Param        req         body     pb.CreateProject_Request{}
// @Success      200         {object} httputil.HTTPResponse{data=pb.CreateProject_Response}
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/projects [post]
func (h *Handler) addProject(c *fiber.Ctx) error {
	input := new(pb.CreateProject_Request)
	c.BodyParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewProjectHandlersClient(h.grpc.Client)

	project, err := rClient.CreateProject(ctx, &pb.CreateProject_Request{
		OwnerId: userID,
		Login:   input.GetLogin(),
		Title:   input.GetTitle(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "Project added", project)
}

// @Summary      Updating project information.
// @Tags         projects
// @Accept       json
// @Produce      json
// @Param        req             body     pb.UpdateProject_Request{}
// @Success      200             {object} httputil.HTTPResponse{data=pb.UpdateProject_Response}
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/projects [patch]
func (h *Handler) patchProject(c *fiber.Ctx) error {
	input := new(pb.UpdateProject_Request)
	c.BodyParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewProjectHandlersClient(h.grpc.Client)

	_, err := rClient.UpdateProject(ctx, &pb.UpdateProject_Request{
		OwnerId:   userID,
		ProjectId: input.GetProjectId(),
		Title:     input.GetTitle(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "Project data updated", nil)
}

// @Summary      Delete project
// @Tags         projects
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true "Project ID"
// @Param        owner_id        path     uuid true "Owner ID"
// @Success      200             {object} httputil.HTTPResponse
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/projects [delete]
func (h *Handler) deleteProject(c *fiber.Ctx) error {
	input := new(pb.DeleteProject_Request)
	c.QueryParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewProjectHandlersClient(h.grpc.Client)

	_, err := rClient.DeleteProject(ctx, &pb.DeleteProject_Request{
		ProjectId: input.GetProjectId(),
		OwnerId:   userID,
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "Project deleted", nil)
}
