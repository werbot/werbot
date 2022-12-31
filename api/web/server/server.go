package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"

	pb_firewall "github.com/werbot/werbot/api/proto/firewall"
	pb "github.com/werbot/werbot/api/proto/server"
)

// @Summary      Show information about server or list of all servers
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        user_id     path     uuid false "Server ID. Parameter Accessible with ROLE_ADMIN rights"
// @Param        server_id   path     uuid false "Server ID"
// @Param        project_id  path     uuid true "Project ID"
// @Success      200         {object} webutil.HTTPResponse{data=pb.ListServer_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers [get]
func (h *handler) getServer(c *fiber.Ctx) error {
	request := new(pb.Server_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.Server_RequestMultiError) {
			e := err.(pb.Server_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewServerHandlersClient(h.Grpc.Client)

	// show all project
	if request.GetServerId() == "" {
		pagination := webutil.GetPaginationFromCtx(c)
		sanitizeSQL, _ := sanitize.SQL(`project_id = $1 AND user_id = $2`, request.GetProjectId(), userID)
		servers, err := rClient.ListServers(ctx, &pb.ListServers_Request{
			Limit:  pagination.GetLimit(),
			Offset: pagination.GetOffset(),
			SortBy: pagination.GetSortBy(),
			Query:  sanitizeSQL,
		})
		if err != nil {
			return webutil.FromGRPC(c, h.log, err)
		}
		if servers.GetTotal() == 0 {
			return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
		}

		return webutil.StatusOK(c, msgServers, servers)
	}

	// show information about the server
	server, err := rClient.Server(ctx, &pb.Server_Request{
		UserId:    userID,
		ServerId:  request.GetServerId(),
		ProjectId: request.GetProjectId(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}
	// if server == nil {
	//	return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	//}

	return webutil.StatusOK(c, msgServers, server)
}

// @Summary      Adding a new server
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     pb.AddServer_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=pb.AddServer_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers [post]
func (h *handler) addServer(c *fiber.Ctx) error {
	request := new(pb.AddServer_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.AddServer_RequestMultiError) {
			e := err.(pb.AddServer_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewServerHandlersClient(h.Grpc.Client)

	server, err := rClient.AddServer(ctx, &pb.AddServer_Request{
		UserId:             userID,
		ProjectId:          request.GetProjectId(),
		Address:            request.GetAddress(),
		Port:               request.GetPort(),
		Login:              request.GetLogin(),
		Title:              request.GetTitle(),
		PrivateDescription: request.GetPrivateDescription(),
		PublicDescription:  request.GetPublicDescription(),
		Auth:               request.GetAuth(),
		Scheme:             request.GetScheme(),
		Audit:              request.GetAudit(),
		Active:             request.GetActive(),
		Password:           request.GetPassword(),
		PublicKey:          request.GetPublicKey(),
		KeyUuid:            request.GetKeyUuid(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgServerAdded, server)
}

// @Summary      Server update
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     pb.UpdateServer_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers [patch]
func (h *handler) patchServer(c *fiber.Ctx) error {
	request := new(pb.UpdateServer_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.UpdateServer_RequestMultiError) {
			e := err.(pb.UpdateServer_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewServerHandlersClient(h.Grpc.Client)

	_, err := rClient.UpdateServer(ctx, &pb.UpdateServer_Request{
		UserId:             userID,
		ServerId:           request.GetServerId(),
		ProjectId:          request.GetProjectId(),
		Address:            request.GetAddress(),
		Port:               request.GetPort(),
		Login:              request.GetLogin(),
		Title:              request.GetTitle(),
		PrivateDescription: request.GetPrivateDescription(),
		PublicDescription:  request.GetPublicDescription(),
		Audit:              request.GetAudit(),
		Active:             request.GetActive(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	// access setting
	access := new(pb.UpdateServerAccess_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := access.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.UpdateServerAccess_RequestMultiError) {
			e := err.(pb.UpdateServerAccess_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	// If the password is not indicated, skip the next step
	if access.Auth == pb.ServerAuth_PASSWORD && access.Password == "" {
		return webutil.StatusOK(c, msgServerUpdated, nil)
	}

	_, err = rClient.UpdateServerAccess(ctx, &pb.UpdateServerAccess_Request{
		UserId:    userID,
		ServerId:  request.GetServerId(),
		ProjectId: request.GetProjectId(),
		Auth:      access.GetAuth(),
		Password:  access.GetPassword(),
		PublicKey: access.GetPublicKey(),
		KeyUuid:   access.GetKeyUuid(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgServerUpdated, nil)
}

// @Summary      Delete server
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        user_id     path     uuid true "User ID"
// @Param        project_id  path     uuid true "Project ID"
// @Param        server_id   path     uuid true "Server ID"
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers [delete]
func (h *handler) deleteServer(c *fiber.Ctx) error {
	request := new(pb.DeleteServer_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.DeleteServer_RequestMultiError) {
			e := err.(pb.DeleteServer_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewServerHandlersClient(h.Grpc.Client)

	_, err := rClient.DeleteServer(ctx, &pb.DeleteServer_Request{
		UserId:    userID,
		ProjectId: request.GetProjectId(),
		ServerId:  request.GetServerId(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgServerDeleted, nil)
}

// @Summary      Get server access
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        user_id     path     uuid true "User ID"
// @Param        project_id  path     uuid true "Project ID"
// @Param        server_id   path     uuid true "Server ID"
// @Success      200         {object} webutil.HTTPResponse{data=pb.ServerAccess_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/access [get]
func (h *handler) getServerAccess(c *fiber.Ctx) error {
	request := new(pb.ServerAccess_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.ServerAccess_RequestMultiError) {
			e := err.(pb.ServerAccess_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewServerHandlersClient(h.Grpc.Client)

	access, err := rClient.ServerAccess(ctx, &pb.ServerAccess_Request{
		UserId:    userID,
		ProjectId: request.GetProjectId(),
		ServerId:  request.GetServerId(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}
	// if access == nil {
	//	return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	//}

	return webutil.StatusOK(c, msgServerAccess, access)
}

// @Summary      Get server activity
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        user_id     path     uuid true "User ID"
// @Param        project_id  path     uuid true "Project ID"
// @Param        server_id   path     uuid true "Server ID"
// @Success      200         {object} webutil.HTTPResponse{data=pb.ServerActivity_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/activity [get]
func (h *handler) getServerActivity(c *fiber.Ctx) error {
	request := new(pb.ServerActivity_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.ServerActivity_RequestMultiError) {
			e := err.(pb.ServerActivity_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewServerHandlersClient(h.Grpc.Client)

	activity, err := rClient.ServerActivity(ctx, &pb.ServerActivity_Request{
		UserId:    userID,
		ServerId:  request.GetServerId(),
		ProjectId: request.GetProjectId(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}
	// if activity == nil {
	// 	return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	// }

	return webutil.StatusOK(c, msgServerActivity, activity)
}

// @Summary      Update server activity
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     pb.UpdateServerActivity_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/activity [patch]
func (h *handler) patchServerActivity(c *fiber.Ctx) error {
	request := new(pb.UpdateServerActivity_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.UpdateServerActivity_RequestMultiError) {
			e := err.(pb.UpdateServerActivity_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewServerHandlersClient(h.Grpc.Client)

	_, err := rClient.UpdateServerActivity(ctx, &pb.UpdateServerActivity_Request{
		UserId:    userID,
		ProjectId: request.GetProjectId(),
		ServerId:  request.GetServerId(),
		Activity:  request.GetActivity(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgServerActivityUpdated, nil)
}

// @Summary      Get server firewall
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     pb.ServerFirewallInfo_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=pb.ServerFirewallInfo_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/firewall [get]
func (h *handler) getServerFirewall(c *fiber.Ctx) error {
	request := new(pb_firewall.ServerFirewall_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb_firewall.ServerFirewall_RequestMultiError) {
			e := err.(pb_firewall.ServerFirewall_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb_firewall.NewFirewallHandlersClient(h.Grpc.Client)

	firewall, err := rClient.ServerFirewall(ctx, &pb_firewall.ServerFirewall_Request{
		UserId:    userID,
		ServerId:  request.GetServerId(),
		ProjectId: request.GetUserId(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgServerFirewall, firewall)
}

// @Summary      Add server firewall
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     pb_firewall.AddServerFirewall_Request
// @Success      200         {object} webutil.HTTPResponse{data=pb_firewall.AddServerFirewall_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/firewall [post]
func (h *handler) postServerFirewall(c *fiber.Ctx) error {
	request := new(pb_firewall.AddServerFirewall_Request)

	if err := protojson.Unmarshal(c.Body(), request); err != nil {
		fmt.Print(err)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb_firewall.AddServerFirewall_RequestMultiError) {
			e := err.(pb_firewall.AddServerFirewall_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb_firewall.NewFirewallHandlersClient(h.Grpc.Client)

	var err error
	response := new(pb_firewall.AddServerFirewall_Response)
	switch record := request.Record.(type) {
	case *pb_firewall.AddServerFirewall_Request_Country:
		response, err = rClient.AddServerFirewall(ctx, &pb_firewall.AddServerFirewall_Request{
			UserId:    userID,
			ProjectId: request.GetProjectId(),
			ServerId:  request.GetServerId(),
			Record: &pb_firewall.AddServerFirewall_Request_Country{
				Country: &pb_firewall.CountryCode{
					Code: record.Country.Code,
				},
			},
		})

	case *pb_firewall.AddServerFirewall_Request_Ip:
		response, err = rClient.AddServerFirewall(ctx, &pb_firewall.AddServerFirewall_Request{
			UserId:    userID,
			ProjectId: request.GetProjectId(),
			ServerId:  request.GetServerId(),
			Record: &pb_firewall.AddServerFirewall_Request_Ip{
				Ip: &pb_firewall.IpMask{
					StartIp: record.Ip.StartIp,
					EndIp:   record.Ip.EndIp,
				},
			},
		})

	default:
		return webutil.StatusBadRequest(c, msgBadRule, nil)
	}

	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgFirewallAdded, response)
}

// @Summary      Status firewall server
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     pb.UpdateAccessPolicy_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/firewall [patch]
func (h *handler) patchAccessPolicy(c *fiber.Ctx) error {
	request := new(pb_firewall.UpdateAccessPolicy_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb_firewall.UpdateAccessPolicy_RequestMultiError) {
			e := err.(pb_firewall.UpdateAccessPolicy_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb_firewall.NewFirewallHandlersClient(h.Grpc.Client)

	_, err := rClient.UpdateAccessPolicy(ctx, &pb_firewall.UpdateAccessPolicy_Request{
		UserId:    userID,
		ProjectId: request.GetProjectId(),
		ServerId:  request.GetServerId(),
		Rule:      request.GetRule(),
		Status:    request.GetStatus(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgFirewallUpdated, nil)
}

// @Summary      Delete server firewall
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     pb_firewall.ServerFirewallInfo_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/firewall [delete]
func (h *handler) deleteServerFirewall(c *fiber.Ctx) error {
	request := new(pb_firewall.DeleteServerFirewall_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb_firewall.DeleteServerFirewall_RequestMultiError) {
			e := err.(pb_firewall.DeleteServerFirewall_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb_firewall.NewFirewallHandlersClient(h.Grpc.Client)

	_, err := rClient.DeleteServerFirewall(ctx, &pb_firewall.DeleteServerFirewall_Request{
		UserId:    userID,
		ProjectId: request.GetProjectId(),
		ServerId:  request.GetServerId(),
		Rule:      request.GetRule(),
		RecordId:  request.GetRecordId(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgFirewallDeleted, nil)
}

// @Summary      Update server status
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     pb.UpdateServerActiveStatus_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/active [patch]
func (h *handler) patchServerStatus(c *fiber.Ctx) error {
	request := new(pb.UpdateServerActiveStatus_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.UpdateServerActiveStatus_RequestMultiError) {
			e := err.(pb.UpdateServerActiveStatus_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewServerHandlersClient(h.Grpc.Client)

	_, err := rClient.UpdateServerActiveStatus(ctx, &pb.UpdateServerActiveStatus_Request{
		UserId:   userID,
		ServerId: request.GetServerId(),
		Status:   request.GetStatus(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	// message section
	message := msgServerIsOnline
	if !request.GetStatus() {
		message = msgServerIsOffline
	}

	return webutil.StatusOK(c, message, nil)
}

// @Summary      Server name by ID
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     pb.ServerNameByID_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/name [get]
func (h *handler) serverNameByID(c *fiber.Ctx) error {
	request := new(pb.ServerNameByID_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.ServerNameByID_RequestMultiError) {
			e := err.(pb.ServerNameByID_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewServerHandlersClient(h.Grpc.Client)

	access, err := rClient.ServerNameByID(ctx, &pb.ServerNameByID_Request{
		UserId:    userID,
		ServerId:  request.GetServerId(),
		ProjectId: request.GetProjectId(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}
	if access == nil {
		return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	}

	return webutil.StatusOK(c, msgServerName, access)
}
