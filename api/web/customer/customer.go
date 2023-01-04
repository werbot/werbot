package customer

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/pkg/webutil"

	pb "github.com/werbot/werbot/api/proto/subscription"
)

// TODO Addition of the API method getCustomer
// subscription information
// request {user_id:1}
// GET /v1/customers
func (h *Handler) getCustomer(c *fiber.Ctx) error {
	request := new(pb.Customer_Request)

	if err := c.BodyParser(&request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.Customer_RequestMultiError) {
			e := err.(pb.Customer_RequestValidationError)
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
	request := new(pb.DeleteCustomer_Request)

	if err := c.BodyParser(&request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.DeleteCustomer_RequestMultiError) {
			e := err.(pb.DeleteCustomer_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	return webutil.StatusOK(c, msgSubscriptionRemoved, request.GetUserId())
}
