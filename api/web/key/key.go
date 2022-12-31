package key

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"

	pb "github.com/werbot/werbot/api/proto/key"
)

// @Summary      Show information about key or list of all keys
// @Tags         key
// @Accept       json
// @Produce      json
// @Param        key_id      path     uuid true "Key ID"
// @Param        user_id     path     uuid true "Key owner ID. Parameter Accessible with ROLE_ADMIN rights"
// @Success      200         {object} webutil.HTTPResponse{data=pb.PublicKey_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/keys [get]
func (h *handler) getKey(c *fiber.Ctx) error {
	request := new(pb.PublicKey_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.PublicKey_RequestMultiError) {
			e := err.(pb.PublicKey_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewKeyHandlersClient(h.Grpc.Client)

	// show all keys
	if request.GetKeyId() == "" {
		pagination := webutil.GetPaginationFromCtx(c)
		sanitizeSQL, _ := sanitize.SQL(`"user"."id" = $1`, userID)
		keys, err := rClient.ListPublicKeys(ctx, &pb.ListPublicKeys_Request{
			Limit:  pagination.GetLimit(),
			Offset: pagination.GetOffset(),
			SortBy: `"user_public_key"."id":ASC`,
			Query:  sanitizeSQL,
		})
		if err != nil {
			return webutil.FromGRPC(c, h.log, err)
		}
		if keys.GetTotal() == 0 {
			return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
		}
		return webutil.StatusOK(c, msgUserKeys, keys)
	}

	// show information about the key
	key, err := rClient.PublicKey(ctx, &pb.PublicKey_Request{
		KeyId:  request.GetKeyId(),
		UserId: userID,
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}
	// if key == nil {
	//	return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	// }

	return webutil.StatusOK(c, msgKeyInfo, key)
}

// @Summary      Adding a new key
// @Tags         key
// @Accept       json
// @Produce      json
// @Param        req         body     pb.AddPublicKey_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/keys [post]
func (h *handler) addKey(c *fiber.Ctx) error {
	request := new(pb.AddPublicKey_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.AddPublicKey_RequestMultiError) {
			e := err.(pb.AddPublicKey_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewKeyHandlersClient(h.Grpc.Client)

	publicKey, err := rClient.AddPublicKey(ctx, &pb.AddPublicKey_Request{
		UserId: userID,
		Title:  request.GetTitle(),
		Key:    request.GetKey(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgKeyAdded, publicKey)
}

// @Summary      Updating a user key by its ID
// @Tags         key
// @Accept       json
// @Produce      json
// @Param        req         body     pb.UpdatePublicKey_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/keys [patch]
func (h *handler) patchKey(c *fiber.Ctx) error {
	request := new(pb.UpdatePublicKey_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.UpdatePublicKey_RequestMultiError) {
			e := err.(pb.UpdatePublicKey_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewKeyHandlersClient(h.Grpc.Client)

	_, err := rClient.UpdatePublicKey(ctx, &pb.UpdatePublicKey_Request{
		KeyId:  request.GetKeyId(),
		UserId: userID,
		Title:  request.GetTitle(),
		Key:    request.GetKey(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgKeyUpdated, nil)
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
func (h *handler) deleteKey(c *fiber.Ctx) error {
	request := new(pb.DeletePublicKey_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.DeletePublicKey_RequestMultiError) {
			e := err.(pb.DeletePublicKey_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewKeyHandlersClient(h.Grpc.Client)

	_, err := rClient.DeletePublicKey(ctx, &pb.DeletePublicKey_Request{
		KeyId:  request.GetKeyId(),
		UserId: userID,
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgKeyRemoved, nil)
}

// @Summary      Generating a New SSH Key Pair
// @Tags         key
// @Accept       json
// @Produce      json
// @Param        req         body     pb.GenerateSSHKey_Request{}
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/keys/generate [get]
func (h *handler) getGenerateNewKey(c *fiber.Ctx) error {
	request := new(pb.GenerateSSHKey_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if request.GetKeyType() == 0 {
		request.KeyType = pb.KeyType_KEY_TYPE_ED25519
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewKeyHandlersClient(h.Grpc.Client)

	key, err := rClient.GenerateSSHKey(ctx, &pb.GenerateSSHKey_Request{
		KeyType: request.GetKeyType(),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgSSHKeyCreated, map[string]string{
		"key_type": key.GetKeyType().String(),
		"uuid":     key.GetUuid(),
		"public":   string(key.GetPublic()),
	})
}
