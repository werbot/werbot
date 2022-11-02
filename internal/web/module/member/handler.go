package member

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/database/sanitize"
	"github.com/werbot/werbot/internal/message"
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
// @Success      200             {object} httputil.HTTPResponse{data=pb.GetMember_Response}
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members [get]
func (h *Handler) getMember(c *fiber.Ctx) error {
	input := new(pb.GetMember_Request)
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
		members, err := rClient.ListMembers(ctx, &pb.ListMembers_Request{
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
		return httputil.StatusOK(c, "List of all members", members)
	}

	// show information about the member
	member, err := rClient.GetMember(ctx, &pb.GetMember_Request{
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

	return httputil.StatusOK(c, "Information about the member", member)
}

// @Summary      Adding a new member on project
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     pb.CreateProject_Request{}
// @Success      200         {object} httputil.HTTPResponse{data=pb.CreateMember_Response}
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/members [post]
func (h *Handler) addMember(c *fiber.Ctx) error {
	input := new(pb.CreateMember_Request)
	c.BodyParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	userID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	member, err := rClient.CreateMember(ctx, &pb.CreateMember_Request{
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
// @Success      200             {object} httputil.HTTPResponse{data=pb.UpdateMember_Response}
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members [patch]
func (h *Handler) patchMember(c *fiber.Ctx) error {
	input := new(pb.UpdateMember_Request)
	c.BodyParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	ownerID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	_, err := rClient.UpdateMember(ctx, &pb.UpdateMember_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		MemberId:  input.GetMemberId(),
		Role:      input.GetRole(),
		Active:    input.GetActive(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "Member data updated", nil)
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
func (h *Handler) deleteMember(c *fiber.Ctx) error {
	input := new(pb.DeleteMember_Request)
	c.QueryParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	ownerID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	_, err := rClient.DeleteMember(ctx, &pb.DeleteMember_Request{
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

// @Summary      Update member status
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     pb.UpdateMemberActiveStatus_Request{}
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/members/active [patch]
func (h *Handler) patchMemberStatus(c *fiber.Ctx) error {
	input := new(pb.UpdateMemberActiveStatus_Request)
	c.BodyParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	ownerID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	_, err := rClient.UpdateMemberActiveStatus(ctx, &pb.UpdateMemberActiveStatus_Request{
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
		message = "Member access is closed"
	}
	return httputil.StatusOK(c, message, nil)
}

////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////

// @Summary      Show information about member or list of all members on server
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true  "Project ID"
// @Param        owner_id        path     uuid false "Project owner ID"
// @Param        server_id       path     uuid false "Server ID on project"
// @Param        member_id       path     uuid false "Member ID. Parameter Accessible with ROLE_ADMIN rights"
// @Success      200             {object} httputil.HTTPResponse
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/server/members [get]
func (h *Handler) getServerMember(c *fiber.Ctx) error {
	input := new(pb.GetServerMember_Request)
	c.QueryParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	ownerID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	// show all member on server
	if input.GetProjectId() == "" {
		pagination := httputil.GetPaginationFromCtx(c)
		members, err := rClient.ListServerMembers(ctx, &pb.ListServerMembers_Request{
			Limit:     pagination.GetLimit(),
			Offset:    pagination.GetOffset(),
			SortBy:    "server_account.id:ASC",
			OwnerId:   ownerID,
			ProjectId: input.GetProjectId(),
			ServerId:  input.GetServerId(),
		})
		if err != nil {
			return httputil.ReturnGRPCError(c, err)
		}
		if members.Total == 0 {
			return httputil.StatusNotFound(c, message.ErrNotFound, nil)
		}

		return httputil.StatusOK(c, "List of all members on server", members)
	}

	member, err := rClient.GetServerMember(ctx, &pb.GetServerMember_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		ServerId:  input.GetServerId(),
		AccountId: input.GetAccountId(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	if member == nil {
		return httputil.StatusNotFound(c, message.ErrNotFound, nil)
	}

	return httputil.StatusOK(c, "Member information on server", member)
}

// @Summary      Adding a new member on server
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     pb.CreateServerMember_Request{}
// @Success      200         {object} httputil.HTTPResponse{data=pb.CreateServerMember_Response}
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/members/server [post]
func (h *Handler) addServerMember(c *fiber.Ctx) error {
	input := new(pb.CreateServerMember_Request)
	c.BodyParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	userID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	member, err := rClient.CreateServerMember(ctx, &pb.CreateServerMember_Request{
		OwnerId:   userID,
		ProjectId: input.GetProjectId(),
		ServerId:  input.GetServerId(),
		MemberId:  input.GetMemberId(),
		Active:    input.GetActive(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "Member added", member)
}

// @Summary      Updating member information on server.
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req             body     pb.UpdateServerMember_Request{}
// @Success      200             {object} httputil.HTTPResponse{data=UpdateServerMember_Response}
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members/server [patch]
func (h *Handler) patchServerMember(c *fiber.Ctx) error {
	input := new(pb.UpdateServerMember_Request)
	c.BodyParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	ownerID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	_, err := rClient.UpdateServerMember(ctx, &pb.UpdateServerMember_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		ServerId:  input.GetServerId(),
		AccountId: input.GetAccountId(),
		Active:    input.GetActive(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "Member server data updated", nil)
}

// @Summary      Delete member on server
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true "Project ID"
// @Param        owner_id        path     uuid true "Owner ID"
// @Param        server_id       path     uuid true "Server ID"
// @Param        account_id      path     uuid true "Account ID"
// @Success      200             {object} httputil.HTTPResponse
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members/server [delete]
func (h *Handler) deleteServerMember(c *fiber.Ctx) error {
	input := new(pb.DeleteServerMember_Request)
	c.QueryParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	ownerID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	_, err := rClient.DeleteServerMember(ctx, &pb.DeleteServerMember_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		ServerId:  input.GetServerId(),
		AccountId: input.GetAccountId(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "Member server deleted", nil)
}

// @Summary      List members without server
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true "Project ID"
// @Param        owner_id        path     uuid true "Owner ID"
// @Param        server_id       path     uuid true "Server ID"
// @Param        name            path     string true "Name"
// @Success      200             {object} httputil.HTTPResponse
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members/server/search [get]
func (h *Handler) getMemberWithoutServer(c *fiber.Ctx) error {
	input := new(pb.GetMemberWithoutServer_Request)
	c.QueryParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	ownerID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	members, err := rClient.GetMemberWithoutServer(ctx, &pb.GetMemberWithoutServer_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		ServerId:  input.GetServerId(),
		Name:      fmt.Sprintf(`%v`, input.GetName()),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "List users without server", members)
}
