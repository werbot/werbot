package customer

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	subscriptionpb "github.com/werbot/werbot/api/proto/subscription"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/pkg/webutil"
)

// TODO Addition of the API method getCustomer
// subscription information
// request {user_id:1}
// GET /v1/customers
func (h *Handler) getCustomer(c *fiber.Ctx) error {
	request := new(subscriptionpb.Customer_Request)

	if err := c.BodyParser(&request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(subscriptionpb.Customer_RequestMultiError) {
			e := err.(subscriptionpb.Customer_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	return webutil.StatusOK(c, msgSubscriptionInfo, request.GetUserId())
}

// TODO Addition of the API method deleteCustomer
// Removing the subscriber
// request {user_id:1}
// DELETE /v1/customers
func (h *Handler) deleteCustomer(c *fiber.Ctx) error {
	request := new(subscriptionpb.DeleteCustomer_Request)

	if err := c.BodyParser(&request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(subscriptionpb.DeleteCustomer_RequestMultiError) {
			e := err.(subscriptionpb.DeleteCustomer_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	return webutil.StatusOK(c, msgSubscriptionRemoved, request.GetUserId())
}
