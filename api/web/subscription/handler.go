package subscription

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/status"

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
func (h *Handler) getSubscriptions(c *fiber.Ctx) error {
	pagination := httputil.GetPaginationFromCtx(c)

	input := userIDReq{}
	c.BodyParser(&input)

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(input.UserID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewSubscriptionHandlersClient(h.grpc.Client)

	sanitizeSQL, _ := sanitize.SQL(`"subscription"."customer_id" = $1`, userID)
	subscriptions, err := rClient.ListSubscriptions(ctx, &pb.ListSubscriptions_Request{
		Limit:  pagination.GetLimit(),
		Offset: pagination.GetOffset(),
		SortBy: pagination.GetSortBy(),
		Query:  sanitizeSQL,
	})
	if err != nil {
		se, _ := status.FromError(err)
		if se.Message() != "" {
			return httputil.StatusBadRequest(c, se.Message(), nil)
		}
		return httputil.InternalServerError(c, internal.ErrUnexpectedError, nil)
	}

	if subscriptions.Total == 0 {
		return httputil.StatusNotFound(c, internal.ErrNotFound, nil)
	}

	return httputil.StatusOK(c, "Subscriptions", subscriptions)
}

// TODO Addition of the API patchSubscription method
// Update user subscription
// request {user_id:1}
// PATCH /v1/subscriptions/:subscription_id
func (h *Handler) patchSubscription(c *fiber.Ctx) error {
	subscriptionID := c.Params("subscription_id")

	var input userIDReq
	if err := c.BodyParser(&input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrBadQueryParams, nil)
	}

	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
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
func (h *Handler) deleteSubscription(c *fiber.Ctx) error {
	subscriptionID := c.Params("subscription_id")

	var input userIDReq
	if err := c.BodyParser(&input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrBadQueryParams, nil)
	}

	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
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
func (h *Handler) stopSubscription(c *fiber.Ctx) error {
	subscriptionID := c.Params("subscription_id")

	var input userIDReq
	if err := c.BodyParser(&input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrBadQueryParams, nil)
	}

	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
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
func (h *Handler) addSubscriptionToUser(c *fiber.Ctx) error {
	subscriptionID := c.Params("subscription_id")

	var input userIDReq
	if err := c.BodyParser(&input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrBadQueryParams, nil)
	}

	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	return httputil.StatusOK(c, "подписка добавлена пользователю", map[string]string{
		"user_id":         input.UserID,
		"subscription_id": subscriptionID,
	})
}
