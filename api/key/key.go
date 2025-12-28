package key

import (
	"github.com/gofiber/fiber/v2"

	event "github.com/werbot/werbot/internal/core/event/recorder"
	keyenum "github.com/werbot/werbot/internal/core/key/proto/enum"
	keymessage "github.com/werbot/werbot/internal/core/key/proto/message"
	keyrpc "github.com/werbot/werbot/internal/core/key/proto/rpc"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Retrieve key
// @Description Retrieves lists all keys if key_id is not provided
// @Tags keys
// @Produce json
// @Param profile_id query string false "Profile UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Param sort_by query string false "Sort by for pagination"
// @Success 200 {object} webutil.HTTPResponse{result=keymessage.Keys_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/keys [get]
func (h *Handler) keys(c *fiber.Ctx) error {
	pagination := webutil.GetPaginationFromCtx(c)
	sessionData := session.AuthProfile(c)
	request := &keymessage.Keys_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		ProfileId: sessionData.ProfileID(c.Query("profile_id")),
		Limit:     pagination.Limit,
		Offset:    pagination.Offset,
		SortBy:    `"updated_at":DESC`,
	}

	rClient := keyrpc.NewKeyHandlersClient(h.Grpc)
	keys, err := rClient.Keys(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(keys)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Keys", result)
}

// @Summary Retrieve keys
// @Description Retrieves a specific key by key_id
// @Tags keys
// @Produce json
// @Param profile_id query string false "Profile UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param key_id path string false "Key UUID"
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Param sort_by query string false "Sort by for pagination"
// @Success 200 {object} webutil.HTTPResponse{result=keymessage.Key_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/keys/{key_id} [get]
func (h *Handler) key(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &keymessage.Key_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		ProfileId: sessionData.ProfileID(c.Query("profile_id")),
		KeyId:     c.Params("key_id"),
	}

	rClient := keyrpc.NewKeyHandlersClient(h.Grpc)
	key, err := rClient.Key(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(key)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Key", result)
}

// @Summary Add a new key
// @Description Adds a new key for the specified profile
// @Tags keys
// @Accept json
// @Produce json
// @Param profile_id query string false "Profile UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param body body keymessage.AddKey_Request true "Add Key Request Body"
// @Success 200 {object} webutil.HTTPResponse{result=keymessage.AddKey_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/keys [post]
func (h *Handler) addKey(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &keymessage.AddKey_Request{
		ProfileId: sessionData.ProfileID(c.Query("profile_id")),
	}

	_ = webutil.Parse(c, request).Body()

	rClient := keyrpc.NewKeyHandlersClient(h.Grpc)
	key, err := rClient.AddKey(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(key)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Profile(request.GetProfileId(), event.ProfileSSHKey, event.OnCreate, request)

	return webutil.StatusOK(c, "Key added", result)
}

// @Summary Update an existing key
// @Description Updates an existing key for the specified profile
// @Tags keys
// @Accept json
// @Produce json
// @Param profile_id query string false "Profile UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param key_id path string true "Key UUID"
// @Param body body keymessage.UpdateKey_Request true "Update Key Request Body"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/keys/{key_id} [patch]
func (h *Handler) updateKey(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &keymessage.UpdateKey_Request{
		ProfileId: sessionData.ProfileID(c.Query("profile_id")),
		KeyId:     c.Params("key_id"),
	}

	_ = webutil.Parse(c, request).Body()

	rClient := keyrpc.NewKeyHandlersClient(h.Grpc)
	if _, err := rClient.UpdateKey(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Profile(request.GetProfileId(), event.ProfileSSHKey, event.OnEdit, request)

	return webutil.StatusOK(c, "Key updated", nil)
}

// @Summary Delete an existing key
// @Description Deletes an existing key for the specified profile
// @Tags keys
// @Produce json
// @Param profile_id query string false "Profile UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param key_id path string true "Key UUID"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/keys/{key_id} [delete]
func (h *Handler) deleteKey(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &keymessage.DeleteKey_Request{
		ProfileId: sessionData.ProfileID(c.Query("profile_id")),
		KeyId:     c.Params("key_id"),
	}

	rClient := keyrpc.NewKeyHandlersClient(h.Grpc)
	if _, err := rClient.DeleteKey(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	ghoster.Secrets(request, false)
	go event.New(h.Grpc).Web(c, sessionData).Profile(request.GetProfileId(), event.ProfileSSHKey, event.OnRemove, request)

	return webutil.StatusOK(c, "Key removed", nil)
}

// @Summary Generate a new SSH key
// @Description Generates a new SSH key of type ed25519 and returns the key details
// @Tags keys
// @Produce json
// @Success 200 {object} webutil.HTTPResponse{result=keymessage.GenerateSSHKey_Response}
// @Failure 500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/keys/generate [get]
func (h *Handler) generateNewKey(c *fiber.Ctx) error {
	request := &keymessage.GenerateSSHKey_Request{
		KeyType: keyenum.KeyType_ed25519,
	}

	rClient := keyrpc.NewKeyHandlersClient(h.Grpc)
	key, err := rClient.GenerateSSHKey(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	key.Passphrase = ""
	result, err := protoutils.ConvertProtoMessageToMap(key)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "New ssh key", result)
}
