package event

import (
	"github.com/gofiber/fiber/v2"

	eventpb "github.com/werbot/werbot/internal/core/event/proto/rpc"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Retrieve event records
// @Description Retrieves event records based on the provided category (profile, project, or scheme) and query parameters
// @Tags event
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param category path string true "Category name (profile, project, scheme)"
// @Param category_id path string true "Name UUID"
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Param sort_by query string false "Sort by for pagination"
// @Success 200 {object} webutil.HTTPResponse{result=eventpb.Events_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/event/{category}/{category_id} [get]
func (h *Handler) events(c *fiber.Ctx) error {
	pagination := webutil.GetPaginationFromCtx(c)
	sessionData := session.AuthProfile(c)
	request := &eventpb.Events_Request{
		IsAdmin: sessionData.IsProfileAdmin(),
		OwnerId: sessionData.ProfileID(c.Query("owner_id")),
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		SortBy:  `"event"."created_at":DESC`,
	}

	// _ = webutil.Parse(c, request).Query()

	switch c.Params("category") {
	case "profile":
		request.RelatedId = &eventpb.Events_Request_ProfileId{ProfileId: c.Params("category_id")}
	case "project":
		request.RelatedId = &eventpb.Events_Request_ProjectId{ProjectId: c.Params("category_id")}
	case "scheme":
		request.RelatedId = &eventpb.Events_Request_SchemeId{SchemeId: c.Params("category_id")}
	}

	rClient := eventpb.NewEventHandlersClient(h.Grpc)
	events, err := rClient.Events(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}
	if events.GetTotal() == 0 {
		return webutil.StatusNotFound(c, nil)
	}

	result, err := protoutils.ConvertProtoMessageToMap(events)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Events", result)
}

// @Summary Retrieve a single event record
// @Description Retrieves a single event record based on the provided category (profile, project, or scheme) and query parameters
// @Tags event
// @Produce json
// @Param category path string true "Category name (profile, project, scheme)"
// @Param category_id path string true "Name UUID"
// @Param event_id path string true "Event UUID"
// @Success 200 {object} webutil.HTTPResponse{result=eventpb.Event_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/event/{category}/{category_id}/{event_id} [get]
func (h *Handler) event(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &eventpb.Event_Request{
		IsAdmin: sessionData.IsProfileAdmin(),
		OwnerId: sessionData.ProfileID(c.Query("owner_id")),
	}

	switch c.Params("category") {
	case "profile":
		request.EventId = &eventpb.Event_Request_ProfileId{ProfileId: c.Params("event_id")}
	case "project":
		request.EventId = &eventpb.Event_Request_ProjectId{ProjectId: c.Params("event_id")}
	case "scheme":
		request.EventId = &eventpb.Event_Request_SchemeId{SchemeId: c.Params("event_id")}
	}

	rClient := eventpb.NewEventHandlersClient(h.Grpc)
	event, err := rClient.Event(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(event)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Event", result)
}
