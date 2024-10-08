package key

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/event"
	keypb "github.com/werbot/werbot/internal/grpc/key/proto/key"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Retrieve key
// @Description Retrieves lists all keys if key_id is not provided
// @Tags keys
// @Produce json
// @Param user_id query string false "User UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Param sort_by query string false "Sort by for pagination"
// @Success 200 {object} webutil.HTTPResponse{result=keypb.Keys_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/keys [get]
func (h *Handler) keys(c *fiber.Ctx) error {
	pagination := webutil.GetPaginationFromCtx(c)
	sessionData := session.AuthUser(c)
	request := &keypb.Keys_Request{
		IsAdmin: sessionData.IsUserAdmin(),
		UserId:  sessionData.UserID(c.Query("user_id")),
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		SortBy:  `"updated_at":DESC`,
	}

	rClient := keypb.NewKeyHandlersClient(h.Grpc)
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
// @Param user_id query string false "User UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param key_id path string false "Key UUID"
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Param sort_by query string false "Sort by for pagination"
// @Success 200 {object} webutil.HTTPResponse{result=keypb.Key_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/keys/{key_id} [get]
func (h *Handler) key(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &keypb.Key_Request{
		IsAdmin: sessionData.IsUserAdmin(),
		UserId:  sessionData.UserID(c.Query("user_id")),
		KeyId:   c.Params("key_id"),
	}

	rClient := keypb.NewKeyHandlersClient(h.Grpc)
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
// @Description Adds a new key for the specified user
// @Tags keys
// @Accept json
// @Produce json
// @Param user_id query string false "User UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param body body keypb.AddKey_Request true "Add Key Request Body"
// @Success 200 {object} webutil.HTTPResponse{result=keypb.AddKey_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/keys [post]
func (h *Handler) addKey(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &keypb.AddKey_Request{
		UserId: sessionData.UserID(c.Query("user_id")),
	}

	_ = webutil.Parse(c, request).Body()

	rClient := keypb.NewKeyHandlersClient(h.Grpc)
	key, err := rClient.AddKey(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(key)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	go event.New(h.Grpc).Web(c, sessionData).Profile(request.GetUserId(), event.ProfileSSHKey, event.OnCreate, request)

	return webutil.StatusOK(c, "Key added", result)
}

// @Summary Update an existing key
// @Description Updates an existing key for the specified user
// @Tags keys
// @Accept json
// @Produce json
// @Param user_id query string false "User UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param key_id path string true "Key UUID"
// @Param body body keypb.UpdateKey_Request true "Update Key Request Body"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/keys/{key_id} [patch]
func (h *Handler) updateKey(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &keypb.UpdateKey_Request{
		UserId: sessionData.UserID(c.Query("user_id")),
		KeyId:  c.Params("key_id"),
	}

	_ = webutil.Parse(c, request).Body()

	rClient := keypb.NewKeyHandlersClient(h.Grpc)
	if _, err := rClient.UpdateKey(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	go event.New(h.Grpc).Web(c, sessionData).Profile(request.GetUserId(), event.ProfileSSHKey, event.OnEdit, request)

	return webutil.StatusOK(c, "Key updated", nil)
}

// @Summary Delete an existing key
// @Description Deletes an existing key for the specified user
// @Tags keys
// @Produce json
// @Param user_id query string false "User UUID". Parameter Accessible with ROLE_ADMIN rights
// @Param key_id path string true "Key UUID"
// @Success 200 {object} webutil.HTTPResponse
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/keys/{key_id} [delete]
func (h *Handler) deleteKey(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &keypb.DeleteKey_Request{
		UserId: sessionData.UserID(c.Query("user_id")),
		KeyId:  c.Params("key_id"),
	}

	rClient := keypb.NewKeyHandlersClient(h.Grpc)
	if _, err := rClient.DeleteKey(c.UserContext(), request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Log the event
	go event.New(h.Grpc).Web(c, sessionData).Profile(request.GetUserId(), event.ProfileSSHKey, event.OnRemove, request)

	return webutil.StatusOK(c, "Key removed", nil)
}

// @Summary Generate a new SSH key
// @Description Generates a new SSH key of type ed25519 and returns the key details
// @Tags keys
// @Produce json
// @Success 200 {object} webutil.HTTPResponse{result=keypb.GenerateSSHKey_Response}
// @Failure 500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/keys/generate [get]
func (h *Handler) generateNewKey(c *fiber.Ctx) error {
	request := &keypb.GenerateSSHKey_Request{
		KeyType: keypb.KeyType_ed25519,
	}

	rClient := keypb.NewKeyHandlersClient(h.Grpc)
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
