package member

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/message"
	"github.com/werbot/werbot/internal/utils/validator"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/internal/grpc/proto/member"
)

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
	if input.GetMemberId() == "" {
		pagination := httputil.GetPaginationFromCtx(c)
		members, err := rClient.ListServerMembers(ctx, &pb.ListServerMembers_Request{
			Limit:     pagination.GetLimit(),
			Offset:    pagination.GetOffset(),
			SortBy:    "server_member.id:ASC",
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

		return httputil.StatusOK(c, "Members on server", members)
	}

	member, err := rClient.GetServerMember(ctx, &pb.GetServerMember_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		ServerId:  input.GetServerId(),
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
	return httputil.StatusOK(c, "Member updated", nil)
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
	return httputil.StatusOK(c, "Member deleted", nil)
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
	return httputil.StatusOK(c, "List members without server", members)
}

// @Summary      Update member status of server
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     pb.UpdateServerMemberStatus_Request{}
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/members/active [patch]
func (h *Handler) patchServerMemberStatus(c *fiber.Ctx) error {
	input := new(pb.UpdateServerMemberStatus_Request)
	c.BodyParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	ownerID := userParameter.GetUserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.grpc.Client)

	_, err := rClient.UpdateServerMemberStatus(ctx, &pb.UpdateServerMemberStatus_Request{
		OwnerId:   ownerID,
		MemberId:  input.GetMemberId(),
		ProjectId: input.GetProjectId(),
		ServerId:  input.GetServerId(),
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
