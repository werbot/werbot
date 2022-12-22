package subscription

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/utils/validate"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/api/proto/subscription"
)

type userIDReq struct {
	UserID string `json:"user_id" validate:"required,uuid"`
}

// @Summary      List of all subscriptions
// @Tags         subscriptions
// @Accept       json
// @Produce      json
// @Param        req         body     userIDReq
// @Success      200         {object} httputil.HTTPResponse(data=GetSubscriptions_Response)
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/subscriptions [get]
func (h *handler) getSubscriptions(c *fiber.Ctx) error {
	pagination := httputil.GetPaginationFromCtx(c)
	input := new(userIDReq)

	if err := c.BodyParser(input); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(input.UserID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewSubscriptionHandlersClient(h.Grpc.Client)

	sanitizeSQL, _ := sanitize.SQL(`"subscription"."customer_id" = $1`, userID)
	subscriptions, err := rClient.ListSubscriptions(ctx, &pb.ListSubscriptions_Request{
		Limit:  pagination.GetLimit(),
		Offset: pagination.GetOffset(),
		SortBy: pagination.GetSortBy(),
		Query:  sanitizeSQL,
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}
	if subscriptions.Total == 0 {
		return httputil.StatusNotFound(c, internal.MsgNotFound, nil)
	}

	return httputil.StatusOK(c, "Subscriptions", subscriptions)
}

// TODO Addition of the API patchSubscription method
// Update user subscription
// request {user_id:1}
// PATCH /v1/subscriptions/:subscription_id
func (h *handler) patchSubscription(c *fiber.Ctx) error {
	var input userIDReq
	subscriptionID := c.Params("subscription_id")

	if err := c.BodyParser(&input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	return httputil.StatusOK(c, "новый ключ обновлен", map[string]string{
		"user_id":         input.UserID,
		"subscription_id": subscriptionID,
	})
}

// TODO add the API deleteSubscription method
// Removing user subscription
// request {user_id:1}
// DELETE /v1/subscriptions/:subscription_id
func (h *handler) deleteSubscription(c *fiber.Ctx) error {
	var input userIDReq
	subscriptionID := c.Params("subscription_id")

	if err := c.BodyParser(&input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	return httputil.StatusOK(c, "подписка удалена", map[string]string{
		"user_id":         input.UserID,
		"subscription_id": subscriptionID,
	})
}

// TODO add the API stopSubscription method
// Stop user subscription
// request {user_id:1}
// POST /v1/subscriptions/:subscription_id/stop
func (h *handler) stopSubscription(c *fiber.Ctx) error {
	var input userIDReq
	subscriptionID := c.Params("subscription_id")

	if err := c.BodyParser(&input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	return httputil.StatusOK(c, "подписка остановлена", map[string]string{
		"user_id":         input.UserID,
		"subscription_id": subscriptionID,
	})
}

// TODO Add API addSubscriptionToUser method
// Adding a new subscription to the user
// request {user_id:1}
// POST /v1/subscriptions/:subscription_id/user
func (h *handler) addSubscriptionToUser(c *fiber.Ctx) error {
	var input userIDReq
	subscriptionID := c.Params("subscription_id")

	if err := c.BodyParser(&input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	return httputil.StatusOK(c, "подписка добавлена пользователю", map[string]string{
		"user_id":         input.UserID,
		"subscription_id": subscriptionID,
	})
}
