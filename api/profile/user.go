package profile

import (
	"github.com/gofiber/fiber/v2"

	profilepb "github.com/werbot/werbot/internal/core/profile/proto/profile"
	"github.com/werbot/werbot/internal/event"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Retrieve profiles
// @Description Retrieves a list of profiles with pagination and sorting options. Access restricted to admin profiles.
// @Tags profiles
// @Accept json
// @Produce json
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Param sort_by query string false "Sort by for pagination"
// @Success 200 {object} webutil.HTTPResponse{result=profilepb.Profiles_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/profiles/list [get]
func (h *Handler) profiles(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)

	// access only for admin
	if !sessionData.IsProfileAdmin() {
		return webutil.StatusNotFound(c, nil)
	}

	pagination := webutil.GetPaginationFromCtx(c)
	request := &profilepb.Profiles_Request{
		IsAdmin: sessionData.IsProfileAdmin(),
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		SortBy:  `"profile"."created_at":ASC`,
	}

	rClient := profilepb.NewProfileHandlersClient(h.Grpc)
	profiles, err := rClient.Profiles(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(profiles)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Profiles", result)
}

// @Summary Retrieve profile
// @Description Retrieves details of a specific profile by profile ID. Access level depends on whether the requester is an admin.
// @Tags profiles
// @Accept json
// @Produce json
// @Param owner_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Success 200 {object} webutil.HTTPResponse{result=profilepb.Profile_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/profiles [get]
func (h *Handler) profile(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &profilepb.Profile_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		ProfileId: sessionData.ProfileID(c.Query("profile_id")),
	}

	rClient := profilepb.NewProfileHandlersClient(h.Grpc)
	profile, err := rClient.Profile(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(profile)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Profile", result)
}

// @Summary Add a new profile
// @Description Adds a new profile to the system. Only accessible by admin profiles.
// @Tags profiles
// @Accept json
// @Produce json
// @Param profile body profilepb.AddProfile_Request true "Add Profile Request"
// @Success 200 {object} webutil.HTTPResponse{result=profilepb.AddProfile_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/profiles [post]
func (h *Handler) addProfile(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)

	// access only for admin
	if !sessionData.IsProfileAdmin() {
		return webutil.StatusNotFound(c, nil)
	}

	request := &profilepb.AddProfile_Request{
		IsAdmin: sessionData.IsProfileAdmin(),
	}

	_ = webutil.Parse(c, request).Body(false)

	rClient := profilepb.NewProfileHandlersClient(h.Grpc)
	profile, err := rClient.AddProfile(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(profile)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Profile(sessionData.Profile.GetProfileId(), event.ProfileProfile, event.OnCreate, request)

	return webutil.StatusOK(c, "Profile added", result)
}

// @Summary Update profile information
// @Description Updates the profile's details based on the provided request data.
// @Tags profiles
// @Accept json
// @Produce json
// @Param profile_id query string false "Owner UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param body body profilepb.UpdateProfile_Request true
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/profiles [put]
func (h *Handler) updateProfile(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &profilepb.UpdateProfile_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		ProfileId: sessionData.ProfileID(c.Query("profile_id")),
	}

	_ = webutil.Parse(c, request).Body(true)

	rClient := profilepb.NewProfileHandlersClient(h.Grpc)
	if _, err := rClient.UpdateProfile(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	var eventType event.EventType
	switch request.GetSetting().(type) {
	case *profilepb.UpdateProfile_Request_Alias, *profilepb.UpdateProfile_Request_Email, *profilepb.UpdateProfile_Request_Name, *profilepb.UpdateProfile_Request_Surname:
		eventType = event.OnUpdate
	case *profilepb.UpdateProfile_Request_Active, *profilepb.UpdateProfile_Request_Confirmed:
		if request.GetActive() || request.GetConfirmed() {
			eventType = event.OnOffline
		} else {
			eventType = event.OnOnline
		}
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Profile(request.GetProfileId(), event.ProfileProfile, eventType, request)

	return webutil.StatusOK(c, "Profile updated", nil)
}

// @Summary Delete Profile
// @Description Deletes a profile either by sending an email with a token (step 1) or by verifying the token and deleting the profile (step 2).
// @Tags profile
// @Accept json
// @Produce json
// @Param profile_id query string true "Profile ID"
// @Param token path string false "Token"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/profiles/delete [post, delete]
func (h *Handler) deleteProfile(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &profilepb.DeleteProfile_Request{
		ProfileId: sessionData.ProfileID(c.Query("profiles")),
	}

	// using on step 1
	if c.Method() == "POST" {
		_ = webutil.Parse(c, request).Body(true)
	}

	// using on step 2
	if c.Method() == "DELETE" {
		request.Request = &profilepb.DeleteProfile_Request_Token{
			Token: c.Params("token"),
		}
	}

	var message, description string
	var eventType event.EventType
	var metaData map[string]any

	switch request.GetRequest().(type) {
	case *profilepb.DeleteProfile_Request_Password: // step 1 - send email and token
		message = "Request for delete"
		description = "An email with instructions to delete your profile has been sent to your email"
		eventType = event.OnMessage
		metaData = event.Metadata{
			"subject": "profile deletion confirmation",
		}
	case *profilepb.DeleteProfile_Request_Token: // step 2 - check token and delete profile
		message = "Profile deleted"
		eventType = event.OnInactive
		metaData = event.Metadata{
			"subject": "profile deleted",
		}
	default:
		return webutil.StatusBadRequest(c, map[string]string{
			"request": "exactly one field is required in oneof",
		})
	}

	rClient := profilepb.NewProfileHandlersClient(h.Grpc)
	if _, err := rClient.DeleteProfile(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	go event.New(h.Grpc).Web(c, sessionData).Profile(request.GetProfileId(), event.ProfileProfile, eventType, metaData)

	return webutil.StatusOK(c, message, description)
}

// @Summary Update profile password
// @Description Updates the password for a given profile ID.
// @Tags profiles
// @Accept json
// @Produce json
// @Param body body profilepb.UpdatePassword_Request true "Update Password Request"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/profiles/password [patch]
func (h *Handler) updatePassword(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &profilepb.UpdatePassword_Request{
		ProfileId: sessionData.Profile.GetProfileId(),
	}

	_ = webutil.Parse(c, request).Body()

	rClient := profilepb.NewProfileHandlersClient(h.Grpc)
	msg, err := rClient.UpdatePassword(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(msg, false)
	go event.New(h.Grpc).Web(c, sessionData).Profile(request.GetProfileId(), event.ProfileProfile, event.OnUpdate, msg)

	return webutil.StatusOK(c, "Password updated", msg)
}
