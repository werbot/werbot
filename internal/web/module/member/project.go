package member

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/message"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/utils/validator"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/internal/grpc/proto/member"
	pb_user "github.com/werbot/werbot/internal/grpc/proto/user"
)

// @Summary      Show information about member or list of all members on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true  "Project ID"
// @Param        owner_id        path     uuid false "Project owner ID"
// @Param        member_id       path     uuid false "Member ID. Parameter Accessible with ROLE_ADMIN rights"
// @Success      200             {object} httputil.HTTPResponse{data=pb.GetProjectMember_Response}
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members [get]
func (h *Handler) getProjectMember(c *fiber.Ctx) error {
	input := new(pb.GetProjectMember_Request)
	c.QueryParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	ownerID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	// show all members
	if input.GetMemberId() == "" {
		pagination := httputil.GetPaginationFromCtx(c)
		sanitizeSQL, _ := sanitize.SQL(`owner_id = $1 AND project_id = $2`, ownerID, input.GetProjectId())
		members, err := rClient.ListProjectMembers(ctx, &pb.ListProjectMembers_Request{
			Limit:  pagination.GetLimit(),
			Offset: pagination.GetOffset(),
			SortBy: "member_id:ASC",
			Query:  sanitizeSQL,
		})
		if err != nil {
			return httputil.ReturnGRPCError(c, err)
		}
		if members.GetTotal() == 0 {
			return httputil.StatusNotFound(c, message.ErrNotFound, nil)
		}
		return httputil.StatusOK(c, "List of members", members)
	}

	// show information about the member
	member, err := rClient.GetProjectMember(ctx, &pb.GetProjectMember_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		MemberId:  input.GetMemberId(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	if member == nil {
		return httputil.StatusNotFound(c, message.ErrNotFound, nil)
	}

	return httputil.StatusOK(c, "Member information", member)
}

// @Summary      Adding a new member on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     pb.CreateProject_Request{}
// @Success      200         {object} httputil.HTTPResponse{data=pb.CreateProjectMember_Response}
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/members [post]
func (h *Handler) addProjectMember(c *fiber.Ctx) error {
	input := new(pb.CreateProjectMember_Request)
	c.BodyParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	userID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	member, err := rClient.CreateProjectMember(ctx, &pb.CreateProjectMember_Request{
		OwnerId:   userID,
		ProjectId: input.GetProjectId(),
		UserId:    input.GetUserId(),
		Role:      pb_user.RoleUser_USER, // TODO directly install the role of the user
		Active:    input.GetActive(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "Member added", member)
}

// @Summary      Updating member information on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req             body     pb.UpdateMember_Request{}
// @Success      200             {object} httputil.HTTPResponse{data=pb.UpdateProjectMember_Response}
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members [patch]
func (h *Handler) patchProjectMember(c *fiber.Ctx) error {
	input := new(pb.UpdateProjectMember_Request)
	c.BodyParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	ownerID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	_, err := rClient.UpdateProjectMember(ctx, &pb.UpdateProjectMember_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		MemberId:  input.GetMemberId(),
		Role:      input.GetRole(),
		Active:    input.GetActive(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "Member updated", nil)
}

// @Summary      Delete member on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true "Project ID"
// @Param        owner_id        path     uuid true "Owner ID"
// @Param        member_id       path     uuid true "Member ID"
// @Success      200             {object} httputil.HTTPResponse
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members [delete]
func (h *Handler) deleteProjectMember(c *fiber.Ctx) error {
	input := new(pb.DeleteProjectMember_Request)
	c.QueryParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	ownerID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	_, err := rClient.DeleteProjectMember(ctx, &pb.DeleteProjectMember_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		MemberId:  input.GetMemberId(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "Member deleted", nil)
}

// @Summary      List users without project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true "Project ID"
// @Param        owner_id        path     uuid true "Owner ID"
// @Param        name            path     string true "Name"
// @Success      200             {object} httputil.HTTPResponse
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members/search [get]
func (h *Handler) getUsersWithoutProject(c *fiber.Ctx) error {
	input := new(pb.ActivityRequest)
	c.QueryParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	ownerID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	members, err := rClient.GetUsersWithoutProject(ctx, &pb.GetUsersWithoutProject_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		Name:      input.GetName(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "List users without project", members)
}

// @Summary      Update member status of project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     pb.UpdateProjectMemberStatus_Request{}
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/members/active [patch]
func (h *Handler) patchProjectMemberStatus(c *fiber.Ctx) error {
	input := new(pb.UpdateProjectMemberStatus_Request)
	c.BodyParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	ownerID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	_, err := rClient.UpdateProjectMemberStatus(ctx, &pb.UpdateProjectMemberStatus_Request{
		OwnerId:   ownerID,
		MemberId:  input.GetMemberId(),
		ProjectId: input.GetProjectId(),
		Status:    input.GetStatus(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}

	// message section
	message := "Member is online"
	if input.GetStatus() == false {
		message = "Member is offline"
	}
	return httputil.StatusOK(c, message, nil)
}
