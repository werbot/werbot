package member

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"

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
// @Success      200             {object} webutil.HTTPResponse
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/server/members [get]
func (h *Handler) getServerMember(c *fiber.Ctx) error {
	request := new(pb.ServerMember_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.ServerMember_RequestMultiError) {
			e := err.(pb.ServerMember_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)

	// show all member on server
	if request.GetMemberId() == "" {
		pagination := webutil.GetPaginationFromCtx(c)
		members, err := rClient.ListServerMembers(ctx, &pb.ListServerMembers_Request{
			Limit:     pagination.GetLimit(),
			Offset:    pagination.GetOffset(),
			SortBy:    "server_member.id:ASC",
			OwnerId:   request.GetOwnerId(),
			ProjectId: request.GetProjectId(),
			ServerId:  request.GetServerId(),
		})
		if err != nil {
			return webutil.FromGRPC(c, h.log, err)
		}

		return webutil.StatusOK(c, msgServerMembers, members)
	}

	member, err := rClient.ServerMember(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}
	if member == nil {
		return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	}

	return webutil.StatusOK(c, msgMemberInfo, member)
}

// @Summary      Adding a new member on server
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     pb.AddServerMember_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=pb.AddServerMember_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/members/server [post]
func (h *Handler) addServerMember(c *fiber.Ctx) error {
	request := new(pb.AddServerMember_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.AddServerMember_RequestMultiError) {
			e := err.(pb.AddServerMember_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)
	member, err := rClient.AddServerMember(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgMemberAdded, member)
}

// @Summary      Updating member information on server.
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req             body     pb.UpdateServerMember_Request{}
// @Success      200             {object} webutil.HTTPResponse{data=UpdateServerMember_Response}
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/members/server [patch]
func (h *Handler) patchServerMember(c *fiber.Ctx) error {
	request := new(pb.UpdateServerMember_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.UpdateServerMember_RequestMultiError) {
			e := err.(pb.UpdateServerMember_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)
	_, err := rClient.UpdateServerMember(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgMemberUpdated, nil)
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
	request := new(pb.DeleteServerMember_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.DeleteServerMember_RequestMultiError) {
			e := err.(pb.DeleteServerMember_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)
	_, err := rClient.DeleteServerMember(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgMemberDeleted, nil)
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
	request := new(pb.MembersWithoutServer_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.MembersWithoutServer_RequestMultiError) {
			e := err.(pb.MembersWithoutServer_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	pagination := webutil.GetPaginationFromCtx(c)
	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())
	request.SortBy = `"user"."name":ASC`
	request.Name = fmt.Sprintf(`%v`, request.GetName())
	request.Limit = pagination.GetLimit()
	request.Offset = pagination.GetOffset()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)
	members, err := rClient.MembersWithoutServer(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgMembersWithoutServer, members)
}

// @Summary      Update member status of server
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        req         body     pb.UpdateServerMemberStatus_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/members/active [patch]
func (h *Handler) patchServerMemberStatus(c *fiber.Ctx) error {
	request := new(pb.UpdateServerMemberStatus_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.UpdateServerMemberStatus_RequestMultiError) {
			e := err.(pb.UpdateServerMemberStatus_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.OwnerId = userParameter.UserID(request.GetOwnerId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewMemberHandlersClient(h.Grpc.Client)
	_, err := rClient.UpdateServerMemberStatus(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	// message section
	message := msgMemberIsOnline
	if !request.GetStatus() {
		message = msgMemberIsOffline
	}

	return webutil.StatusOK(c, message, nil)
}
