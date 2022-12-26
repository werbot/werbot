package subscription

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/storage/postgres/sanitize"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/api/proto/subscription"
)

// @Summary      List of all subscriptions
// @Tags         subscriptions
// @Accept       json
// @Produce      json
// @Param        req         body     userIDReq
// @Success      200         {object} httputil.HTTPResponse(data=GetSubscriptions_Response)
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/subscriptions [get]
func (h *handler) getSubscriptions(c *fiber.Ctx) error {
	request := new(pb.ListSubscriptions_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.ListSubscriptions_RequestMultiError) {
			e := err.(pb.ListSubscriptions_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewSubscriptionHandlersClient(h.Grpc.Client)

	pagination := httputil.GetPaginationFromCtx(c)
	sanitizeSQL, _ := sanitize.SQL(`"subscription"."customer_id" = $1`, userID)
	subscriptions, err := rClient.ListSubscriptions(ctx, &pb.ListSubscriptions_Request{
		Limit:  pagination.GetLimit(),
		Offset: pagination.GetOffset(),
		SortBy: pagination.GetSortBy(),
		Query:  sanitizeSQL,
	})
	if err != nil {
		return httputil.FromGRPC(c, h.log, err)
	}
	if subscriptions.Total == 0 {
		return httputil.StatusNotFound(c, internal.MsgNotFound, nil)
	}

	return httputil.StatusOK(c, "subscriptions", subscriptions)
}

// TODO Addition of the API patchSubscription method
// Update user subscription
// request {user_id:1}
// PATCH /v1/subscriptions/:subscription_id
func (h *handler) patchSubscription(c *fiber.Ctx) error {
	request := new(pb.UpdateSubscription_Request)
	request.SubscriptionId = c.Params("subscription_id")

	if err := c.BodyParser(&request); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.ListSubscriptions_RequestMultiError) {
			e := err.(pb.ListSubscriptions_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	return httputil.StatusOK(c, "update", map[string]string{
		"user_id":         request.GetCustomerId(),
		"subscription_id": request.GetSubscriptionId(),
	})
}

// TODO add the API deleteSubscription method
// Removing user subscription
// request {user_id:1}
// DELETE /v1/subscriptions/:subscription_id
func (h *handler) deleteSubscription(c *fiber.Ctx) error {
	request := new(pb.DeleteSubscription_Request)
	request.SubscriptionId = c.Params("subscription_id")

	if err := c.BodyParser(&request); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.DeleteSubscription_RequestMultiError) {
			e := err.(pb.DeleteSubscription_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	return httputil.StatusOK(c, "deleted", map[string]string{
		"subscription_id": request.GetSubscriptionId(),
	})
}

/*
// TODO add the API stopSubscription method
// Stop user subscription
// request {user_id:1}
// POST /v1/subscriptions/:subscription_id/stop
func (h *handler) stopSubscription(c *fiber.Ctx) error {
	request := new(userIDReq)
	subscriptionID := c.Params("subscription_id")

	if err := c.BodyParser(&request); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(request); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	return httputil.StatusOK(c, "stop", map[string]string{
		"user_id":         request.UserID,
		"subscription_id": subscriptionID,
	})
}

// TODO Add API addSubscriptionToUser method
// Adding a new subscription to the user
// request {user_id:1}
// POST /v1/subscriptions/:subscription_id/user
func (h *handler) addSubscriptionToUser(c *fiber.Ctx) error {
	request := new(userIDReq)
	subscriptionID := c.Params("subscription_id")

	if err := c.BodyParser(&request); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(request); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	return httputil.StatusOK(c, "subscription", map[string]string{
		"user_id":         request.UserID,
		"subscription_id": subscriptionID,
	})
}
*/
