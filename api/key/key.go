package key

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal/grpc"
	keypb "github.com/werbot/werbot/internal/grpc/key/proto"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/trace"
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
	request := new(keypb.Key_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInvalidArgument(c)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := keypb.NewKeyHandlersClient(h.Grpc)

	// show all keys
	if request.GetKeyId() == "" {
		pagination := webutil.GetPaginationFromCtx(c)
		sanitizeSQL, err := sanitize.SQL(`"user"."id" = $1`,
			request.GetUserId(),
		)
		keys, err := rClient.ListKeys(ctx, &keypb.ListKeys_Request{
			Limit:  pagination.GetLimit(),
			Offset: pagination.GetOffset(),
			SortBy: `"user_public_key"."id":ASC`,
			Query:  sanitizeSQL,
		})
		if err != nil {
			return webutil.FromGRPC(c, err)
		}
		if keys.GetTotal() == 0 {
			return webutil.FromGRPC(c, status.Error(codes.NotFound, "Not found"))
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
	request := new(keypb.AddKey_Request)

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
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
	request := new(keypb.UpdateKey_Request)

	if err := c.BodyParser(request); err != nil {
		return webutil.FromGRPC(c, trace.Error(codes.InvalidArgument))
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := keypb.NewKeyHandlersClient(h.Grpc)
	if _, err := rClient.UpdateKey(ctx, request); err != nil {
		return webutil.FromGRPC(c, err)
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
	request := new(keypb.DeleteKey_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInvalidArgument(c)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := keypb.NewKeyHandlersClient(h.Grpc)
	if _, err := rClient.DeleteKey(ctx, request); err != nil {
		return webutil.FromGRPC(c, err)
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
	request := new(keypb.GenerateSSHKey_Request)

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

	return webutil.StatusOK(c, "ssh key created", map[string]string{
		"key_type": key.GetKeyType().String(),
		"uuid":     key.GetUuid(),
		"public":   string(key.GetPublic()),
	})
}
