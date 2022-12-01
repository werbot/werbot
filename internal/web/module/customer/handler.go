package customer

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/utils/validator"
	"github.com/werbot/werbot/internal/web/httputil"
)

type userReq struct {
	UserID int32 `json:"user_id" validate:"required,numeric"`
}

// TODO Addition of the API method getCustomer
// subscription information
// request {user_id:1}
// GET /v1/customers
func (h *Handler) getCustomer(c *fiber.Ctx) error {
	var input userReq
	if err := c.BodyParser(&input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrBadQueryParams, nil)
	}

	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	return httputil.StatusOK(c, "Information about the subscription", input.UserID)
}

// Todo Addition of the API method deleteCustomer
// Removing the subscriber
// request {user_id:1}
// DELETE /v1/customers
func (h *Handler) deleteCustomer(c *fiber.Ctx) error {
	var input userReq
	if err := c.BodyParser(&input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrBadQueryParams, nil)
	}

	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	return httputil.StatusOK(c, "The subscriber is deleted", input.UserID)
}
