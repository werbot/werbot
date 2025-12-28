package member

import (
	"github.com/gofiber/fiber/v2"

	event "github.com/werbot/werbot/internal/core/event/recorder"
	memberrpc "github.com/werbot/werbot/internal/core/member/proto/rpc"
	membermessage "github.com/werbot/werbot/internal/core/member/proto/message"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Retrieve scheme members or search profiles without a scheme
// @Description Retrieves the list of members for a specified scheme, or searches for profiles without a scheme if the "search" addon is provided
// @Tags members
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param scheme_id path string true "Scheme UUID"
// @Param addon path string false "Addon (e.g., 'search')"
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Param sort_by query string false "Sort by for pagination"
// @Success 200 {object} webutil.HTTPResponse{result1=membermessage.MembersWithoutScheme_Response,result2=membermessage.SchemeMembers_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/scheme/{scheme_id}/{addon}? [get]
func (h *Handler) schemeMembers(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	pagination := webutil.GetPaginationFromCtx(c)

	// search profiles without scheme
	if c.Params("addon") == "search" {
		request := &membermessage.MembersWithoutScheme_Request{
			OwnerId:  sessionData.ProfileID(c.Query("owner_id")),
			SchemeId: c.Params("scheme_id"),
			Limit:    pagination.Limit,
			Offset:   pagination.Offset,
			SortBy:   `"profile"."name":ASC`,
			Alias:    c.Query("alias"),
		}

		rClient := memberrpc.NewMemberHandlersClient(h.Grpc)
		members, err := rClient.MembersWithoutScheme(c.UserContext(), request)
		if err != nil {
			return webutil.FromGRPC(c, err)
		}

		result, err := protoutils.ConvertProtoMessageToMap(members)
		if err != nil {
			return webutil.FromGRPC(c, err)
		}

		return webutil.StatusOK(c, "Members without scheme", result)
	}

	// default show
	request := &membermessage.SchemeMembers_Request{
		IsAdmin:  sessionData.IsProfileAdmin(),
		OwnerId:  sessionData.ProfileID(c.Query("owner_id")),
		SchemeId: c.Params("scheme_id"),
		Limit:    pagination.Limit,
		Offset:   pagination.Offset,
		SortBy:   "scheme_member.id:ASC",
	}

	rClient := memberrpc.NewMemberHandlersClient(h.Grpc)
	members, err := rClient.SchemeMembers(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(members)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Members", result)
}

// @Summary Retrieve scheme member information
// @Description Retrieves the details of a specific member within a specified scheme
// @Tags members
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param scheme_id path string true "Scheme UUID"
// @Param scheme_member_id path string true "Member UUID"
// @Success 200 {object} webutil.HTTPResponse{result=membermessage.SchemeMember_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/scheme/{scheme_id}/{scheme_member_id} [get]
func (h *Handler) schemeMember(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &membermessage.SchemeMember_Request{
		IsAdmin:        sessionData.IsProfileAdmin(),
		OwnerId:        sessionData.ProfileID(c.Query("owner_id")),
		SchemeId:       c.Params("scheme_id"),
		SchemeMemberId: c.Params("scheme_member_id"),
	}

	rClient := memberrpc.NewMemberHandlersClient(h.Grpc)
	member, err := rClient.SchemeMember(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(member)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Member", result)
}

// @Summary Add a new member to a scheme
// @Description Adds a new member to the specified scheme using owner_id, scheme_id, and member_id parameters
// @Tags members
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param scheme_id path string true "Scheme UUID"
// @Param member_id path string true "Member UUID"
// @Success 200 {object} webutil.HTTPResponse{result=membermessage.AddSchemeMember_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/scheme/{scheme_id} [post]
func (h *Handler) addSchemeMember(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &membermessage.AddSchemeMember_Request{
		OwnerId:  sessionData.ProfileID(c.Query("owner_id")),
		SchemeId: c.Params("scheme_id"),
	}

	_ = webutil.Parse(c, request).Body(true)

	rClient := memberrpc.NewMemberHandlersClient(h.Grpc)
	member, err := rClient.AddSchemeMember(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(member)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Scheme(request.GetOwnerId(), event.SchemeMember, event.OnCreate, request)

	return webutil.StatusOK(c, "Member added", result)
}

// @Summary Update scheme member details
// @Description Updates the details of an existing member in the specified scheme using owner_id, scheme_id, and member_id parameters
// @Tags members
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param scheme_id path string true "Scheme UUID"
// @Param scheme_member_id path string true "Member UUID"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/scheme/{scheme_id}/{scheme_member_id} [patch]
func (h *Handler) updateSchemeMember(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &membermessage.UpdateSchemeMember_Request{
		OwnerId:        sessionData.ProfileID(c.Query("owner_id")),
		SchemeId:       c.Params("scheme_id"),
		SchemeMemberId: c.Params("scheme_member_id"),
	}

	_ = webutil.Parse(c, request).Body(true)

	rClient := memberrpc.NewMemberHandlersClient(h.Grpc)
	if _, err := rClient.UpdateSchemeMember(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	var eventType event.Type
	switch request.GetSetting().(type) {
	case *membermessage.UpdateSchemeMember_Request_Active:
		eventType = event.OnActive
	case *membermessage.UpdateSchemeMember_Request_Online:
		if request.GetActive() {
			eventType = event.OnOffline
		} else {
			eventType = event.OnOnline
		}
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Scheme(request.GetOwnerId(), event.SchemeMember, eventType, request)

	return webutil.StatusOK(c, "Member updated", nil)
}

// @Summary Delete a scheme member
// @Description Deletes a member from a specified scheme
// @Tags Scheme Members
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param scheme_id path string true "UUID of the scheme"
// @Param scheme_member_id path string true "UUID of the member to be deleted"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/members/scheme/{scheme_id}/{scheme_member_id} [delete]
func (h *Handler) deleteSchemeMember(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &membermessage.DeleteSchemeMember_Request{
		OwnerId:        sessionData.ProfileID(c.Query("owner_id")),
		SchemeId:       c.Params("scheme_id"),
		SchemeMemberId: c.Params("scheme_member_id"),
	}

	rClient := memberrpc.NewMemberHandlersClient(h.Grpc)
	if _, err := rClient.DeleteSchemeMember(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Scheme(request.GetOwnerId(), event.SchemeMember, event.OnRemove, request)

	return webutil.StatusOK(c, "Member deleted", nil)
}
