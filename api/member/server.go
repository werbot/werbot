package member

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
	memberpb "github.com/werbot/werbot/internal/grpc/member/proto"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"
)

// @Summary      Show information about member or list of all members on server
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true  "Project ID"
// @Param        owner_id        path     uuid false "Project owner ID"
// @Param        server_id       path     uuid false "Server ID on project"
// @Param        member_id       path     uuid false "Member ID. Parameter Accessible with ROLE_ADMIN rights"
// @Success      200             {object} webutil.HTTPResponse
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/server/members [get]
func (h *Handler) getServerMember(c *fiber.Ctx) error {
	request := &memberpb.ServerMember_Request{}

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, nil)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)

	// show all member on server
	if request.GetMemberId() == "" {
		pagination := webutil.GetPaginationFromCtx(c)
		members, err := rClient.ListServerMembers(ctx, &memberpb.ListServerMembers_Request{
			Limit:     pagination.Limit,
			Offset:    pagination.Offset,
			SortBy:    "server_member.id:ASC",
			OwnerId:   request.GetOwnerId(),
			ProjectId: request.GetProjectId(),
			ServerId:  request.GetServerId(),
		})
		if err != nil {
			return webutil.FromGRPC(c, err)
		}

		return webutil.StatusOK(c, "server members", members)
	}

	member, err := rClient.ServerMember(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}
	if member == nil {
		return webutil.StatusNotFound(c, nil)
	}

	return webutil.StatusOK(c, "member information", member)
}

// @Summary      Adding a new member on server
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     memberpb.AddServerMember_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=memberpb.AddServerMember_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/members/server [post]
func (h *Handler) addServerMember(c *fiber.Ctx) error {
	request := &memberpb.AddServerMember_Request{}

	if err := c.BodyParser(request); err != nil {
		return webutil.StatusBadRequest(c, "The body of the request is damaged")
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	member, err := rClient.AddServerMember(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "member added", member)
}

// @Summary      Updating member information on server.
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req             body     memberpb.UpdateServerMember_Request{}
// @Success      200             {object} webutil.HTTPResponse{data=UpdateServerMember_Response}
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/members/server [patch]
func (h *Handler) updateServerMember(c *fiber.Ctx) error {
	request := &memberpb.UpdateServerMember_Request{}

	if err := c.BodyParser(request); err != nil {
		return webutil.StatusBadRequest(c, "The body of the request is damaged")
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	if _, err := rClient.UpdateServerMember(ctx, request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "member updated", nil)
}

// @Summary      Delete member on server
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true "Project ID"
// @Param        owner_id        path     uuid true "Owner ID"
// @Param        server_id       path     uuid true "Server ID"
// @Param        member_id       path     uuid true "Member ID"
// @Success      200             {object} webutil.HTTPResponse
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/members/server [delete]
func (h *Handler) deleteServerMember(c *fiber.Ctx) error {
	request := &memberpb.DeleteServerMember_Request{}

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, nil)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	if _, err := rClient.DeleteServerMember(ctx, request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "member deleted", nil)
}

// @Summary      List members without server
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        project_id      path     uuid true "Project ID"
// @Param        owner_id        path     uuid true "Owner ID"
// @Param        server_id       path     uuid true "Server ID"
// @Param        name            path     string true "Name"
// @Success      200             {object} webutil.HTTPResponse
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/members/server/search [get]
func (h *Handler) getMembersWithoutServer(c *fiber.Ctx) error {
	request := &memberpb.MembersWithoutServer_Request{}

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, nil)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	pagination := webutil.GetPaginationFromCtx(c)
	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())
	request.SortBy = `"user"."name":ASC`
	request.Login = fmt.Sprintf(`%v`, request.GetLogin())
	request.Limit = pagination.Limit
	request.Offset = pagination.Offset

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	members, err := rClient.MembersWithoutServer(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "members without server", members)
}

// @Summary      Update member status of server
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     memberpb.UpdateServerMember_Request{data=memberpb.UpdateServerMember_Request_Active}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/members/active [patch]
func (h *Handler) updateServerMemberActive(c *fiber.Ctx) error {
	request := &memberpb.UpdateServerMember_Request{}
	request.Setting = &memberpb.UpdateServerMember_Request_Active{}

	if err := c.BodyParser(request); err != nil {
		return webutil.StatusBadRequest(c, "The body of the request is damaged")
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := memberpb.NewMemberHandlersClient(h.Grpc)
	if _, err := rClient.UpdateServerMember(ctx, request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// message section
	message := "member is online"
	if !request.GetActive() {
		message = "member is offline"
	}

	return webutil.StatusOK(c, message, nil)
}
