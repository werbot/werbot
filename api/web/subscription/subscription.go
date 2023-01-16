package subscription

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	subscriptionpb "github.com/werbot/werbot/api/proto/subscription"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"
)

// @Summary      List of all subscriptions
// @Tags         subscriptions
// @Accept       json
// @Produce      json
// @Param        req         body     userIDReq
// @Success      200         {object} webutil.HTTPResponse(data=GetSubscriptions_Response)
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/subscriptions [get]
func (h *Handler) getSubscriptions(c *fiber.Ctx) error {
	request := new(subscriptionpb.ListSubscriptions_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(subscriptionpb.ListSubscriptions_RequestMultiError) {
			e := err.(subscriptionpb.ListSubscriptions_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	pagination := webutil.GetPaginationFromCtx(c)
	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())
	request.Limit = pagination.GetLimit()
	request.Offset = pagination.GetOffset()
	request.SortBy = pagination.GetSortBy()
	request.Query, _ = sanitize.SQL(`"subscription"."customer_id" = $1`,
		request.GetUserId(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := subscriptionpb.NewSubscriptionHandlersClient(h.Grpc.Client)
	subscriptions, err := rClient.ListSubscriptions(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}
	if subscriptions.Total == 0 {
		return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	}

	return webutil.StatusOK(c, msgSubscriptions, subscriptions)
}

// TODO Addition of the API patchSubscription method
// Update user subscription
// request {user_id:1}
// PATCH /v1/subscriptions/:subscription_id
func (h *Handler) patchSubscription(c *fiber.Ctx) error {
	request := new(subscriptionpb.UpdateSubscription_Request)
	request.SubscriptionId = c.Params("subscription_id")

	if err := c.BodyParser(&request); err != nil {
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(subscriptionpb.ListSubscriptions_RequestMultiError) {
			e := err.(subscriptionpb.ListSubscriptions_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	return webutil.StatusOK(c, msgSubscriptionUpdated, map[string]string{
		"user_id":         request.GetCustomerId(),
		"subscription_id": request.GetSubscriptionId(),
	})
}

// TODO add the API deleteSubscription method
// Removing user subscription
// request {user_id:1}
// DELETE /v1/subscriptions/:subscription_id
func (h *Handler) deleteSubscription(c *fiber.Ctx) error {
	request := new(subscriptionpb.DeleteSubscription_Request)
	request.SubscriptionId = c.Params("subscription_id")

	if err := c.BodyParser(&request); err != nil {
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(subscriptionpb.DeleteSubscription_RequestMultiError) {
			e := err.(subscriptionpb.DeleteSubscription_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	return webutil.StatusOK(c, msgSubscriptionDeleted, map[string]string{
		"subscription_id": request.GetSubscriptionId(),
	})
}

/*
// TODO add the API stopSubscription method
// Stop user subscription
// request {user_id:1}
// POST /v1/subscriptions/:subscription_id/stop
func (h *Handler) stopSubscription(c *fiber.Ctx) error {
	request := new(userIDReq)
	subscriptionID := c.Params("subscription_id")

	if err := c.BodyParser(&request); err != nil {
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(request); err != nil {
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	return webutil.StatusOK(c, "stop", map[string]string{
		"user_id":         request.UserID,
		"subscription_id": subscriptionID,
	})
}

// TODO Add API addSubscriptionToUser method
// Adding a new subscription to the user
// request {user_id:1}
// POST /v1/subscriptions/:subscription_id/user
func (h *Handler) addSubscriptionToUser(c *fiber.Ctx) error {
	request := new(userIDReq)
	subscriptionID := c.Params("subscription_id")

	if err := c.BodyParser(&request); err != nil {
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(request); err != nil {
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	return webutil.StatusOK(c, "subscription", map[string]string{
		"user_id":         request.UserID,
		"subscription_id": subscriptionID,
	})
}
*/
