package key

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/message"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/utils/validator"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/internal/grpc/proto/key"
)

// @Summary      Show information about key or list of all keys
// @Tags         key
// @Accept       json
// @Produce      json
// @Param        key_id      path     uuid true "Key ID"
// @Param        user_id     path     uuid true "Key owner ID. Parameter Accessible with ROLE_ADMIN rights"
// @Success      200         {object} httputil.HTTPResponse{data=pb.GetPublicKey_Response}
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/keys [get]
func (h *Handler) getKey(c *fiber.Ctx) error {
	input := new(pb.GetPublicKey_Request)
	c.QueryParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	userID := userParameter.GetUserID(input.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewKeyHandlersClient(h.grpc.Client)

	// show all keys
	if input.GetKeyId() == "" {
		pagination := httputil.GetPaginationFromCtx(c)
		sanitizeSQL, _ := sanitize.SQL(`"user"."id" = $1`, userID)
		keys, err := rClient.ListPublicKeys(ctx, &pb.ListPublicKeys_Request{
			Limit:  pagination.GetLimit(),
			Offset: pagination.GetOffset(),
			SortBy: `"user_public_key"."id":ASC`,
			Query:  sanitizeSQL,
		})
		if err != nil {
			return httputil.ReturnGRPCError(c, err)
		}
		if keys.GetTotal() == 0 {
			return httputil.StatusNotFound(c, message.ErrNotFound, nil)
		}
		return httputil.StatusOK(c, "User keys", keys)
	}

	// show information about the key
	key, err := rClient.GetPublicKey(ctx, &pb.GetPublicKey_Request{
		KeyId:  input.GetKeyId(),
		UserId: userID,
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	if key == nil {
		return httputil.StatusNotFound(c, message.ErrNotFound, nil)
	}

	return httputil.StatusOK(c, "Key information", key)
}

// @Summary      Adding a new key
// @Tags         key
// @Accept       json
// @Produce      json
// @Param        req         body     pb.CreatePublicKey_Request{}
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/keys [post]
func (h *Handler) addKey(c *fiber.Ctx) error {
	input := new(pb.CreatePublicKey_Request)
	c.BodyParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	userID := userParameter.GetUserID(input.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewKeyHandlersClient(h.grpc.Client)

	publicKey, err := rClient.CreatePublicKey(ctx, &pb.CreatePublicKey_Request{
		UserId: userID,
		Title:  input.GetTitle(),
		Key:    input.GetKey(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "New key added", publicKey)
}

// @Summary      Updating a user key by its ID
// @Tags         key
// @Accept       json
// @Produce      json
// @Param        req         body     pb.UpdatePublicKey_Request{}
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/keys [patch]
func (h *Handler) patchKey(c *fiber.Ctx) error {
	input := new(pb.UpdatePublicKey_Request)
	c.BodyParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	userID := userParameter.GetUserID(input.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewKeyHandlersClient(h.grpc.Client)

	_, err := rClient.UpdatePublicKey(ctx, &pb.UpdatePublicKey_Request{
		KeyId:  input.GetKeyId(),
		UserId: userID,
		Title:  input.GetTitle(),
		Key:    input.GetKey(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}

	return httputil.StatusOK(c, "User key data updated", nil)
}

// @Summary      Deleting a user key by its ID
// @Tags         key
// @Accept       json
// @Produce      json
// @Param        key_id      path     uuid true "key_id"
// @Param        user_id     path     uuid true "user_id"
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/keys [delete]
func (h *Handler) deleteKey(c *fiber.Ctx) error {
	input := new(pb.DeletePublicKey_Request)
	c.QueryParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	userID := userParameter.GetUserID(input.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewKeyHandlersClient(h.grpc.Client)

	_, err := rClient.DeletePublicKey(ctx, &pb.DeletePublicKey_Request{
		KeyId:  input.GetKeyId(),
		UserId: userID,
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}
	return httputil.StatusOK(c, "User key removed", nil)
}

// @Summary      Generating a New SSH Key Pair
// @Tags         key
// @Accept       json
// @Produce      json
// @Param        req         body     pb.GenerateSSHKey_Request{}
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/keys/generate [get]
func (h *Handler) getGenerateNewKey(c *fiber.Ctx) error {
	input := new(pb.GenerateSSHKey_Request)
	c.BodyParser(input)

	if input.GetKeyType() == 0 {
		input.KeyType = pb.KeyType_KEY_TYPE_ED25519
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewKeyHandlersClient(h.grpc.Client)

	key, err := rClient.GenerateSSHKey(ctx, &pb.GenerateSSHKey_Request{
		KeyType: input.GetKeyType(),
	})
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}

	return httputil.StatusOK(c, "SSH key pair created", map[string]string{
		"key_type": key.GetKeyType().String(),
		"uuid":     key.GetUuid(),
		"public":   string(key.GetPublic()),
	})
}
