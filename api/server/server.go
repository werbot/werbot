package server

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	firewallpb "github.com/werbot/werbot/internal/grpc/firewall/proto"
	serverpb "github.com/werbot/werbot/internal/grpc/server/proto"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"
)

// @Summary      Show information about server or list of all servers
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        user_id     path     uuid false "Server ID. Parameter Accessible with ROLE_ADMIN rights"
// @Param        server_id   path     uuid false "Server ID"
// @Param        project_id  path     uuid true "Project ID"
// @Success      200         {object} webutil.HTTPResponse{data=serverpb.ListServer_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers [get]
func (h *Handler) server(c *fiber.Ctx) error {
	request := new(serverpb.Server_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.FromGRPC(c, errors.New("incorrect parameters"))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(serverpb.Server_RequestMultiError) {
			e := err.(serverpb.Server_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := serverpb.NewServerHandlersClient(h.Grpc.Client)

	// show all project
	if request.GetServerId() == "" {
		pagination := webutil.GetPaginationFromCtx(c)
		sanitizeSQL, _ := sanitize.SQL(`project_id = $1 AND user_id = $2`,
			request.GetProjectId(),
			request.GetUserId(),
		)
		servers, err := rClient.ListServers(ctx, &serverpb.ListServers_Request{
			Limit:  pagination.GetLimit(),
			Offset: pagination.GetOffset(),
			SortBy: pagination.GetSortBy(),
			Query:  sanitizeSQL,
		})
		if err != nil {
			return webutil.FromGRPC(c, err)
		}
		if servers.GetTotal() == 0 {
			return webutil.FromGRPC(c, status.Error(codes.NotFound, "not found"))
		}

		return webutil.StatusOK(c, "servers", servers)
	}

	// show information about the server
	server, err := rClient.Server(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}
	// if server == nil {
	//	return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	//}

	return webutil.StatusOK(c, "servers", server)
}

// @Summary      Adding a new server
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     serverpb.AddServer_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=serverpb.AddServer_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers [post]
func (h *Handler) addServer(c *fiber.Ctx) error {
	request := new(serverpb.AddServer_Request)

	// Deciding what to add
	if !json.Valid(c.Body()) {
		return webutil.FromGRPC(c, errors.New("incorrect parameters"))
	}

	var _request map[string]map[string]any
	json.Unmarshal(c.Body(), &_request)
	keys := reflect.ValueOf(_request["access"]).MapKeys()

	if len(keys) == 0 {
		return webutil.FromGRPC(c, errors.New("the type of authorization is not chosen"))
	}

	switch keys[0].String() {
	case "key":
		request.Access = new(serverpb.AddServer_Request_Key)
	case "password":
		request.Access = new(serverpb.AddServer_Request_Password)
	default:
		return webutil.FromGRPC(c, errors.New("failed to validate struct")) // MsgFailedToValidateStruct
	}
	// -----------------------

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(serverpb.AddServer_RequestMultiError) {
			e := err.(serverpb.AddServer_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := serverpb.NewServerHandlersClient(h.Grpc.Client)
	server, err := rClient.AddServer(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "server added", server)
}

// @Summary      Server update
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     serverpb.UpdateServer_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers [patch]
func (h *Handler) updateServer(c *fiber.Ctx) error {
	request := new(serverpb.UpdateServer_Request)

	// Deciding what to update
	if !json.Valid(c.Body()) {
		return webutil.FromGRPC(c, errors.New("incorrect parameters"))
	}

	var update map[string]map[string]any
	json.Unmarshal(c.Body(), &update)
	keys := reflect.ValueOf(update["setting"]).MapKeys()

	switch keys[0].String() {
	case "info":
		request.Setting = new(serverpb.UpdateServer_Request_Info)
	case "audit":
		request.Setting = new(serverpb.UpdateServer_Request_Audit)
	case "active":
		request.Setting = new(serverpb.UpdateServer_Request_Active)
	case "online":
		request.Setting = new(serverpb.UpdateServer_Request_Online)
	default:
		return webutil.FromGRPC(c, errors.New("incorrect parameters"))
	}
	// -----------------------

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(serverpb.UpdateServer_RequestMultiError) {
			e := err.(serverpb.UpdateServer_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := serverpb.NewServerHandlersClient(h.Grpc.Client)
	_, err := rClient.UpdateServer(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "server updated", nil)
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
func (h *Handler) deleteServer(c *fiber.Ctx) error {
	request := new(serverpb.DeleteServer_Request)

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(serverpb.DeleteServer_RequestMultiError) {
			e := err.(serverpb.DeleteServer_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := serverpb.NewServerHandlersClient(h.Grpc.Client)
	_, err := rClient.DeleteServer(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "server deleted", nil)
}

// @Summary      Get server access
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        user_id     path     uuid true "User ID"
// @Param        project_id  path     uuid true "Project ID"
// @Param        server_id   path     uuid true "Server ID"
// @Success      200         {object} webutil.HTTPResponse{data=serverpb.ServerAccess_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/access [get]
func (h *Handler) serverAccess(c *fiber.Ctx) error {
	request := new(serverpb.ServerAccess_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.FromGRPC(c, errors.New("incorrect parameters"))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(serverpb.ServerAccess_RequestMultiError) {
			e := err.(serverpb.ServerAccess_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := serverpb.NewServerHandlersClient(h.Grpc.Client)
	access, err := rClient.ServerAccess(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}
	// if access == nil {
	//	return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	//}

	return webutil.StatusOK(c, "server access", access)
}

// @Summary      Update server access
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     serverpb.UpdateServerAccess_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/access [patch]
func (h *Handler) updateServerAccess(c *fiber.Ctx) error {
	request := new(serverpb.UpdateServerAccess_Request)

	// Deciding what to access
	if !json.Valid(c.Body()) {
		return webutil.FromGRPC(c, errors.New("incorrect parameters")) // MsgFailedToValidateStruct
	}

	var update map[string]map[string]any
	json.Unmarshal(c.Body(), &update)
	keys := reflect.ValueOf(update["access"]).MapKeys()

	switch keys[0].String() {
	case "password":
		request.Access = new(serverpb.UpdateServerAccess_Request_Password)
	case "key":
		request.Access = new(serverpb.UpdateServerAccess_Request_Key)
	default:
		return webutil.FromGRPC(c, errors.New("incorrect parameters"))
	}
	// -----------------------

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(serverpb.UpdateServerAccess_RequestMultiError) {
			e := err.(serverpb.UpdateServerAccess_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())
	request.ServerId = request.GetServerId()
	request.ProjectId = request.GetProjectId()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := serverpb.NewServerHandlersClient(h.Grpc.Client)
	_, err := rClient.UpdateServerAccess(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "server updated", nil)
}

// @Summary      Get server activity
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        user_id     path     uuid true "User ID"
// @Param        project_id  path     uuid true "Project ID"
// @Param        server_id   path     uuid true "Server ID"
// @Success      200         {object} webutil.HTTPResponse{data=serverpb.ServerActivity_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/activity [get]
func (h *Handler) serverActivity(c *fiber.Ctx) error {
	request := new(serverpb.ServerActivity_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.FromGRPC(c, errors.New("incorrect parameters"))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(serverpb.ServerActivity_RequestMultiError) {
			e := err.(serverpb.ServerActivity_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := serverpb.NewServerHandlersClient(h.Grpc.Client)
	activity, err := rClient.ServerActivity(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}
	// if activity == nil {
	// 	return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	// }

	return webutil.StatusOK(c, "server activity", activity)
}

// @Summary      Update server activity
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     serverpb.UpdateServerActivity_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/activity [patch]
func (h *Handler) updateServerActivity(c *fiber.Ctx) error {
	request := new(serverpb.UpdateServerActivity_Request)

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(serverpb.UpdateServerActivity_RequestMultiError) {
			e := err.(serverpb.UpdateServerActivity_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := serverpb.NewServerHandlersClient(h.Grpc.Client)
	_, err := rClient.UpdateServerActivity(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "server activity updated", nil)
}

// @Summary      Get server firewall
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     firewallpb.ServerFirewallInfo_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=firewallpb.ServerFirewallInfo_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/firewall [get]
func (h *Handler) serverFirewall(c *fiber.Ctx) error {
	request := new(firewallpb.ServerFirewall_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.FromGRPC(c, errors.New("incorrect parameters"))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(firewallpb.ServerFirewall_RequestMultiError) {
			e := err.(firewallpb.ServerFirewall_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := firewallpb.NewFirewallHandlersClient(h.Grpc.Client)
	firewall, err := rClient.ServerFirewall(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "server firewall", firewall)
}

// @Summary      Add server firewall
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     firewallpb.AddServerFirewall_Request
// @Success      200         {object} webutil.HTTPResponse{data=firewallpb.AddServerFirewall_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/firewall [post]
func (h *Handler) addServerFirewall(c *fiber.Ctx) error {
	request := new(firewallpb.AddServerFirewall_Request)

	if err := protojson.Unmarshal(c.Body(), request); err != nil {
		h.log.Error(err).Send()
		return webutil.FromGRPC(c, errors.New("incorrect parameters"))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(firewallpb.AddServerFirewall_RequestMultiError) {
			e := err.(firewallpb.AddServerFirewall_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := firewallpb.NewFirewallHandlersClient(h.Grpc.Client)

	var err error
	switch request.Record.(type) {
	case *firewallpb.AddServerFirewall_Request_CountryCode:
		record := new(firewallpb.AddServerFirewall_Request_CountryCode)
		record.CountryCode = request.GetCountryCode()
		request.Record = record

	case *firewallpb.AddServerFirewall_Request_Ip:
		record := new(firewallpb.AddServerFirewall_Request_Ip)
		record.Ip = new(firewallpb.IpMask)
		record.Ip.StartIp = request.GetIp().GetStartIp()
		record.Ip.EndIp = request.GetIp().GetEndIp()
		request.Record = record

	default:
		return webutil.FromGRPC(c, errors.New("bad rule"))
	}

	response := new(firewallpb.AddServerFirewall_Response)
	response, err = rClient.AddServerFirewall(ctx, request)

	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "firewall added", response)
}

// @Summary      Status firewall server
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     firewallpb.UpdateServerFirewall_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/firewall [patch]
func (h *Handler) updateServerFirewall(c *fiber.Ctx) error {
	request := new(firewallpb.UpdateServerFirewall_Request)

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(firewallpb.UpdateServerFirewall_RequestMultiError) {
			e := err.(firewallpb.UpdateServerFirewall_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := firewallpb.NewFirewallHandlersClient(h.Grpc.Client)
	_, err := rClient.UpdateServerFirewall(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "firewall updated", nil)
}

// @Summary      Delete server firewall
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     firewallpb.ServerFirewallInfo_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/firewall [delete]
func (h *Handler) deleteServerFirewall(c *fiber.Ctx) error {
	request := new(firewallpb.DeleteServerFirewall_Request)

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(firewallpb.DeleteServerFirewall_RequestMultiError) {
			e := err.(firewallpb.DeleteServerFirewall_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := firewallpb.NewFirewallHandlersClient(h.Grpc.Client)
	_, err := rClient.DeleteServerFirewall(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "firewall deleted", nil)
}

// @Summary      Server name by ID
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     serverpb.ServerNameByID_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/name [get]
func (h *Handler) serverNameByID(c *fiber.Ctx) error {
	request := new(serverpb.ServerNameByID_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.FromGRPC(c, errors.New("incorrect parameters"))
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(serverpb.ServerNameByID_RequestMultiError) {
			e := err.(serverpb.ServerNameByID_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.FromGRPC(c, err, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := serverpb.NewServerHandlersClient(h.Grpc.Client)
	access, err := rClient.ServerNameByID(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}
	if access == nil {
		return webutil.FromGRPC(c, status.Error(codes.NotFound, "not found"))
	}

	return webutil.StatusOK(c, "server name", access)
}
