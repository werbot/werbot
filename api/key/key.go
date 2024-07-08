package key

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
	eventpb "github.com/werbot/werbot/internal/grpc/event/proto"
	keypb "github.com/werbot/werbot/internal/grpc/key/proto"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"
)

// @Summary      Show information about key or list of all keys
// @Tags         key
// @Accept       json
// @Produce      json
// @Param        key_id      path     uuid true "Key ID"
// @Param        user_id     path     uuid true "Key owner ID. Parameter Accessible with ROLE_ADMIN rights"
// @Success      200         {object} webutil.HTTPResponse{data=keypb.Key_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/keys [get]
func (h *Handler) getKey(c *fiber.Ctx) error {
	request := &keypb.Key_Request{}

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, nil)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := keypb.NewKeyHandlersClient(h.Grpc)

	// show all keys
	if request.GetKeyId() == "" {
		pagination := webutil.GetPaginationFromCtx(c)
		keys, err := rClient.ListKeys(ctx, &keypb.ListKeys_Request{
			UserId: request.GetUserId(),
			Limit:  pagination.Limit,
			Offset: pagination.Offset,
			SortBy: `"user_public_key"."id":ASC`,
		})
		if err != nil {
			return webutil.FromGRPC(c, err)
		}
		if keys.GetTotal() == 0 {
			return webutil.StatusNotFound(c, nil)
		}

		return webutil.StatusOK(c, "user keys", keys)
	}

	// show information about the key
	key, err := rClient.Key(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}
	// if key == nil {
	//	return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	// }

	return webutil.StatusOK(c, "key information", key)
}

// @Summary      Adding a new key
// @Tags         key
// @Accept       json
// @Produce      json
// @Param        req         body     keypb.AddKey_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/keys [post]
func (h *Handler) addKey(c *fiber.Ctx) error {
	request := &keypb.AddKey_Request{}

	if err := c.BodyParser(request); err != nil {
		return webutil.StatusBadRequest(c, "The body of the request is damaged")
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := keypb.NewKeyHandlersClient(h.Grpc)
	publicKey, err := rClient.AddKey(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// add event in log
	rClientEvent := eventpb.NewEventHandlersClient(h.Grpc)
	_, err = rClientEvent.AddEvent(ctx, &eventpb.AddEvent_Request{
		Section: &eventpb.AddEvent_Request_Profile{
			Profile: &eventpb.Profile{
				Id:      request.UserId,
				Section: eventpb.Profile_ssh_key,
			},
		},
		UserAgent: string(c.Request().Header.UserAgent()),
		Ip:        c.IP(),
		Event:     eventpb.EventType_onCreate,
		MetaData:  []byte(`{"action":"key added"}`),
	})
	if err != nil {
		h.log.Error(err).Send()
	}

	return webutil.StatusOK(c, "key added", publicKey)
}

// @Summary      Updating a user key by its ID
// @Tags         key
// @Accept       json
// @Produce      json
// @Param        req         body     keypb.UpdateKey_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/keys [patch]
func (h *Handler) updateKey(c *fiber.Ctx) error {
	request := &keypb.UpdateKey_Request{}

	if err := c.BodyParser(request); err != nil {
		return webutil.StatusBadRequest(c, "The body of the request is damaged")
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := keypb.NewKeyHandlersClient(h.Grpc)
	if _, err := rClient.UpdateKey(ctx, request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// add event in log
	rClientEvent := eventpb.NewEventHandlersClient(h.Grpc)
	_, err := rClientEvent.AddEvent(ctx, &eventpb.AddEvent_Request{
		Section: &eventpb.AddEvent_Request_Profile{
			Profile: &eventpb.Profile{
				Id:      request.UserId,
				Section: eventpb.Profile_ssh_key,
			},
		},
		UserAgent: string(c.Request().Header.UserAgent()),
		Ip:        c.IP(),
		Event:     eventpb.EventType_onEdit,
	})
	if err != nil {
		h.log.Error(err).Send()
	}

	return webutil.StatusOK(c, "key updated", nil)
}

// @Summary      Deleting a user key by its ID
// @Tags         key
// @Accept       json
// @Produce      json
// @Param        key_id      path     uuid true "key_id"
// @Param        user_id     path     uuid true "user_id"
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/keys [delete]
func (h *Handler) deleteKey(c *fiber.Ctx) error {
	request := &keypb.DeleteKey_Request{}

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, nil)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := keypb.NewKeyHandlersClient(h.Grpc)
	if _, err := rClient.DeleteKey(ctx, request); err != nil {
		return webutil.FromGRPC(c, err)
	}

	// add event in log
	rClientEvent := eventpb.NewEventHandlersClient(h.Grpc)
	_, err := rClientEvent.AddEvent(ctx, &eventpb.AddEvent_Request{
		Section: &eventpb.AddEvent_Request_Profile{
			Profile: &eventpb.Profile{
				Id:      request.UserId,
				Section: eventpb.Profile_ssh_key,
			},
		},
		UserAgent: string(c.Request().Header.UserAgent()),
		Ip:        c.IP(),
		Event:     eventpb.EventType_onRemove,
		MetaData:  []byte(`{"action":"key removed"}`),
	})
	if err != nil {
		h.log.Error(err).Send()
	}

	return webutil.StatusOK(c, "key removed", nil)
}

// @Summary      Generating a New SSH Key Pair
// @Tags         key
// @Accept       json
// @Produce      json
// @Param        req         body     keypb.GenerateSSHKey_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/keys/generate [get]
func (h *Handler) getGenerateNewKey(c *fiber.Ctx) error {
	request := &keypb.GenerateSSHKey_Request{}

	if request.GetKeyType() == 0 {
		request.KeyType = keypb.KeyType_ed25519
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := keypb.NewKeyHandlersClient(h.Grpc)
	key, err := rClient.GenerateSSHKey(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "New ssh key", map[string]string{
		"key_type": key.GetKeyType().String(),
		"uuid":     key.GetUuid(),
		"public":   string(key.GetPublic()),
	})
}
