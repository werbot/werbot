package member

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/utils/validate"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/api/proto/member"
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
func (h *handler) getServerMember(c *fiber.Ctx) error {
	input := new(pb.GetServerMember_Request)
	c.QueryParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.AuthUser(c)
	ownerID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

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
		return httputil.StatusNotFound(c, internal.ErrNotFound, nil)
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
func (h *handler) addServerMember(c *fiber.Ctx) error {
	input := new(pb.CreateServerMember_Request)
	c.BodyParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

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
func (h *handler) patchServerMember(c *fiber.Ctx) error {
	input := new(pb.UpdateServerMember_Request)
	c.BodyParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.AuthUser(c)
	ownerID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

	_, err := rClient.UpdateServerMember(ctx, &pb.UpdateServerMember_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		ServerId:  input.GetServerId(),
		MemberId:  input.GetMemberId(),
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
// @Param        member_id       path     uuid true "Member ID"
// @Success      200             {object} httputil.HTTPResponse
// @Failure      400,401,404,500 {object} httputil.HTTPResponse
// @Router       /v1/members/server [delete]
func (h *handler) deleteServerMember(c *fiber.Ctx) error {
	input := new(pb.DeleteServerMember_Request)
	c.QueryParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.AuthUser(c)
	ownerID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

	_, err := rClient.DeleteServerMember(ctx, &pb.DeleteServerMember_Request{
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		ServerId:  input.GetServerId(),
		MemberId:  input.GetMemberId(),
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
func (h *handler) getMembersWithoutServer(c *fiber.Ctx) error {
	input := new(pb.GetMembersWithoutServer_Request)
	c.QueryParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.AuthUser(c)
	ownerID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

	pagination := httputil.GetPaginationFromCtx(c)
	members, err := rClient.GetMembersWithoutServer(ctx, &pb.GetMembersWithoutServer_Request{
		Limit:     pagination.GetLimit(),
		Offset:    pagination.GetOffset(),
		SortBy:    "\"user\".\"name\":ASC",
		OwnerId:   ownerID,
		ProjectId: input.GetProjectId(),
		ServerId:  input.GetServerId(),
		Name:      fmt.Sprintf(`%v`, input.GetName()),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "Members without server", members)
}

// @Summary      Update member status of server
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     pb.UpdateServerMemberStatus_Request{}
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/members/active [patch]
func (h *handler) patchServerMemberStatus(c *fiber.Ctx) error {
	input := new(pb.UpdateServerMemberStatus_Request)
	c.BodyParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.AuthUser(c)
	ownerID := userParameter.UserID(input.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

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
