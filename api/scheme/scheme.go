package scheme

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/event"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/mathutil"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
	"github.com/werbot/werbot/pkg/utils/strutil"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Retrieve scheme information
// @Description Get a list of schemes based on owner or project details with pagination.
// @Tags schemes
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID". One is server|database|desktop|container|cloud|application
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Param sort_by query string false "Sort by for pagination"
// @Success 200 {object} webutil.HTTPResponse{result=schemepb.Schemes_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/schemes/{project_id}/{scheme_type} [get]
func (h *Handler) schemes(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	pagination := webutil.GetPaginationFromCtx(c)
	request := &schemepb.Schemes_Request{
		IsAdmin:    sessionData.IsUserAdmin(),
		OwnerId:    sessionData.UserID(c.Query("owner_id")),
		ProjectId:  c.Params("project_id"),
		SchemeType: schemeaccesspb.SchemeType(schemeaccesspb.SchemeType_value[c.Params("scheme_type")]),
		Limit:      pagination.Limit,
		Offset:     pagination.Offset,
		SortBy:     "id:ASC",
	}

	rClient := schemepb.NewSchemeHandlersClient(h.Grpc)
	schemes, err := rClient.Schemes(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	schemeGroupe := mathutil.RoundToHundred(schemeaccesspb.SchemeType_value[c.Params("scheme_type")])
	schemeName := fmt.Sprintf(`%ss`, strutil.CapitalizeFirstLetter(schemeaccesspb.SchemeType_name[schemeGroupe]))

	result, err := protoutils.ConvertProtoMessageToMap(schemes)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, schemeName, result)
}

// @Summary Retrieve scheme details
// @Description Get detailed information about a specific scheme based on owner or project details.
// @Tags schemes
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param scheme_id path string true "Scheme UUID"
// @Success 200 {object} webutil.HTTPResponse{result=schemepb.Scheme_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/schemes/{project_id}/{scheme_id} [get]
func (h *Handler) scheme(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &schemepb.Scheme_Request{
		IsAdmin:   sessionData.IsUserAdmin(),
		OwnerId:   sessionData.UserID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		SchemeId:  c.Params("scheme_id"),
	}

	rClient := schemepb.NewSchemeHandlersClient(h.Grpc)
	scheme, err := rClient.Scheme(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(scheme)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Scheme", result)
}

// @Summary Add a new scheme
// @Description Adds a new scheme for a given owner and project.
// @Tags schemes
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param body body schemepb.AddScheme_Request true "Add Scheme Request Body"
// @Success 200 {object} webutil.HTTPResponse{result=schemepb.AddScheme_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/schemes/{project_id}/{scheme} [post]
func (h *Handler) addScheme(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &schemepb.AddScheme_Request{
		OwnerId:   sessionData.UserID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
	}

	_ = webutil.Parse(c, request).Body(true)

	rClient := schemepb.NewSchemeHandlersClient(h.Grpc)
	scheme, err := rClient.AddScheme(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(scheme)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Scheme(request.GetOwnerId(), event.SchemeScheme, event.OnCreate, request)

	return webutil.StatusOK(c, "Scheme added", result)
}

// @Summary Update scheme
// @Description Update the details of an existing scheme.
// @Tags schemes
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param scheme_id path string true "Scheme UUID"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/schemes/{project_id} [put]
func (h *Handler) updateScheme(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &schemepb.UpdateScheme_Request{
		OwnerId:   sessionData.UserID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		SchemeId:  c.Params("scheme_id"),
	}

	_ = webutil.Parse(c, request).Body(true)

	rClient := schemepb.NewSchemeHandlersClient(h.Grpc)
	if _, err := rClient.UpdateScheme(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	var eventType event.EventType
	switch request.GetSetting().(type) {
	case *schemepb.UpdateScheme_Request_Title, *schemepb.UpdateScheme_Request_Description, *schemepb.UpdateScheme_Request_Scheme:
		eventType = event.OnUpdate
	case *schemepb.UpdateScheme_Request_Audit, *schemepb.UpdateScheme_Request_Active:
		if request.GetAudit() || request.GetActive() || request.GetOnline() {
			eventType = event.OnOffline
		} else {
			eventType = event.OnOnline
		}
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Scheme(request.GetOwnerId(), event.SchemeSetting, eventType, request)

	return webutil.StatusOK(c, "Scheme updated", nil)
}

// @Summary Delete a scheme
// @Description Deletes a scheme based on owner UUID, project UUID, and scheme UUID provided in the request.
// @Tags schemes
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param scheme_id path string true "Scheme UUID"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/schemes/{project_id}/{scheme_id} [delete]
func (h *Handler) deleteScheme(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &schemepb.DeleteScheme_Request{
		OwnerId:   sessionData.UserID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		SchemeId:  c.Params("scheme_id"),
	}

	rClient := schemepb.NewSchemeHandlersClient(h.Grpc)
	if _, err := rClient.DeleteScheme(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Scheme(request.GetOwnerId(), event.SchemeSetting, event.OnRemove, request)

	return webutil.StatusOK(c, "Scheme deleted", nil)
}

// @Summary Get scheme access
// @Description Retrieves access details for a scheme based on owner UUID, project UUID, and scheme UUID provided in the request.
// @Tags schemes
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param scheme_id path string true "Scheme UUID"
// @Success 200 {object} webutil.HTTPResponse{result=schemepb.SchemeAccess_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/schemes/{project_id}/{scheme_id}/access [get]
func (h *Handler) schemeAccess(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &schemepb.SchemeAccess_Request{
		OwnerId:   sessionData.UserID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		SchemeId:  c.Params("scheme_id"),
	}

	rClient := schemepb.NewSchemeHandlersClient(h.Grpc)
	access, err := rClient.SchemeAccess(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(access)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Scheme access", result)
}

// @Summary Retrieve scheme activity
// @Description Retrieves the activity details of a specific scheme based on provided parameters.
// @Tags schemes
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param scheme_id path string true "Scheme UUID"
// @Param timestamp path string false "Timestamp (optional, can be 'now' or a Unix timestamp)"
// @Success 200 {object} webutil.HTTPResponse{result=schemepb.SchemeActivity_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/schemes/{project_id}/{scheme_id}/activity/{timestamp} [get]
func (h *Handler) schemeActivity(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &schemepb.SchemeActivity_Request{
		OwnerId:   sessionData.UserID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		SchemeId:  c.Params("scheme_id"),
	}

	if timestampParam := c.Params("timestamp"); timestampParam != "" {
		switch timestampParam {
		case "now":
			request.Timestamp = timestamppb.Now()
		default:
			if timestamp, err := c.ParamsInt("timestamp"); err != nil {
				return webutil.FromGRPC(c, err)
			} else {
				t := time.Unix(int64(timestamp), 0)
				request.Timestamp = timestamppb.New(t)
			}
		}
	}

	rClient := schemepb.NewSchemeHandlersClient(h.Grpc)
	activity, err := rClient.SchemeActivity(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(activity)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Scheme activity", result)
}

// @Summary Update Scheme Activity
// @Description Updates the activity details of a specific scheme within a project.
// @Tags schemes
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param scheme_id path string true "Scheme UUID"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/schemes/{project_id}/{scheme_id}/activity [put]
func (h *Handler) updateSchemeActivity(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &schemepb.UpdateSchemeActivity_Request{
		OwnerId:   sessionData.UserID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		SchemeId:  c.Params("scheme_id"),
	}

	_ = webutil.Parse(c, request).Body()

	rClient := schemepb.NewSchemeHandlersClient(h.Grpc)
	if _, err := rClient.UpdateSchemeActivity(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Scheme(request.GetOwnerId(), event.SchemeActivity, event.OnUpdate, request)

	return webutil.StatusOK(c, "Scheme activity updated", nil)
}

// @Summary Retrieve Scheme Firewall
// @Description Get firewall details for a specified scheme including country and network information.
// @Tags schemes
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param scheme_id path string true "Scheme UUID"
// @Success 200 {object} webutil.HTTPResponse{result=schemepb.SchemeFirewall_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/schemes/{project_id}/{scheme_id}/firewall [get]
func (h *Handler) schemeFirewall(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &schemepb.SchemeFirewall_Request{
		OwnerId:   sessionData.UserID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		SchemeId:  c.Params("scheme_id"),
	}

	rClient := schemepb.NewSchemeHandlersClient(h.Grpc)
	firewall, err := rClient.SchemeFirewall(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(firewall)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Scheme firewall", result)
}

// @Summary Add a new scheme
// @Description Adds a new scheme for a given owner and project.
// @Tags schemes
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param body body schemepb.AddScheme_Request true "Add Scheme Request Body"
// @Success 200 {object} webutil.HTTPResponse{result=schemepb.AddSchemeFirewall_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/schemes/{project_id}/{scheme_id}/firewall [post]
func (h *Handler) addSchemeFirewall(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &schemepb.AddSchemeFirewall_Request{
		OwnerId:   sessionData.UserID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		SchemeId:  c.Params("scheme_id"),
	}

	_ = webutil.Parse(c, request).Body(true)

	rClient := schemepb.NewSchemeHandlersClient(h.Grpc)
	firewall, err := rClient.AddSchemeFirewall(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(firewall)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	var recordType string
	switch request.GetRecord().(type) {
	case *schemepb.AddSchemeFirewall_Request_CountryCode:
		recordType = "Country"
	case *schemepb.AddSchemeFirewall_Request_Network:
		recordType = "Network"
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Scheme(request.GetOwnerId(), event.SchemeFirewall, event.OnCreate, request)

	recordName := fmt.Sprintf(`%s added`, recordType)
	return webutil.StatusOK(c, recordName, result)
}

// @Summary Update Scheme Firewall
// @Description Updates the firewall settings for a specific scheme in a project.
// @Tags schemes
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param body body schemepb.UpdateSchemeFirewall_Request true "Add Scheme Request Body"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/schemes/{project_id}/{scheme_id}/firewall [put]
func (h *Handler) updateSchemeFirewall(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &schemepb.UpdateSchemeFirewall_Request{
		OwnerId:   sessionData.UserID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		SchemeId:  c.Params("scheme_id"),
	}

	_ = webutil.Parse(c, request).Body(true)

	rClient := schemepb.NewSchemeHandlersClient(h.Grpc)
	if _, err := rClient.UpdateSchemeFirewall(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Scheme(request.GetOwnerId(), event.SchemeFirewall, event.OnUpdate, request)

	return webutil.StatusOK(c, "Record updated", nil)
}

// @Summary Delete a scheme
// @Description Deletes a scheme based on owner UUID, project UUID, and scheme UUID provided in the request.
// @Tags schemes
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param project_id path string true "Project UUID"
// @Param scheme_id path string true "Scheme UUID"
// @Param firewall_type path string true "Type of firewall rule (country/network)"
// @Param firewall_id path string true "ID of the firewall rule to delete"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/schemes/{project_id}/{scheme_id}/firewalls/{firewall_type}/{firewall_id} [delete]
func (h *Handler) deleteSchemeFirewall(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &schemepb.DeleteSchemeFirewall_Request{
		OwnerId:   sessionData.UserID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		SchemeId:  c.Params("scheme_id"),
	}

	firewallType := c.Params("firewall_type")
	switch firewallType {
	case "country":
		request.Record = &schemepb.DeleteSchemeFirewall_Request_CountryId{
			CountryId: c.Params("firewall_id"),
		}
	case "network":
		request.Record = &schemepb.DeleteSchemeFirewall_Request_NetworkId{
			NetworkId: c.Params("firewall_id"),
		}
	}

	rClient := schemepb.NewSchemeHandlersClient(h.Grpc)
	if _, err := rClient.DeleteSchemeFirewall(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Scheme(request.GetOwnerId(), event.SchemeFirewall, event.OnRemove, request)

	recordName := fmt.Sprintf(`%s deleted`, strutil.CapitalizeFirstLetter(firewallType))
	return webutil.StatusOK(c, recordName, nil)
}
