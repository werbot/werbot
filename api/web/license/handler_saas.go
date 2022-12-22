//go:build saas

package license

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/utils/validate"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/api/proto/license"
)

type licenseInput struct {
	License string `json:"license" validate:"required,base64"`
}

// @Summary      License expired info
// @Tags         license
// @Accept       json
// @Produce      json
// @Param        req         body     licenseInput
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/license/expired [get]
func (h *handler) getLicenseExpired(c *fiber.Ctx) error {
	input := new(licenseInput)

	if err := c.BodyParser(input); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	licenseDec, err := base64.StdEncoding.DecodeString(input.License)
	if err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgBadRequest, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewLicenseHandlersClient(h.Grpc.Client)

	expiredLic, err := rClient.LicenseExpired(ctx, &pb.LicenseExpired_Request{
		License: licenseDec,
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}

	return httputil.StatusOK(c, "License expired", expiredLic.Status)
}

// @Summary      Creating a new license
// @Tags         license
// @Accept       json
// @Produce      json
// @Param        req     body     pb.AddLicenseRequest
// @Success      200     {object} httputil.HTTPResponse
// @Failure      400,500 {object} httputil.HTTPResponse
// @Router       /v1/license [post]
func (h *handler) postLicense(c *fiber.Ctx) error {
	input := new(pb.AddLicense_Request)

	if err := c.BodyParser(input); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	dataLicense := &pb.AddLicense_Request{
		Ip:    input.GetIp(),
		Token: input.GetToken(),
	}

	userParameter := middleware.AuthUser(c)
	if userParameter.IsUserAdmin() {
		dataLicense = &pb.AddLicense_Request{
			Customer:   input.GetCustomer(),
			Subscriber: input.GetSubscriber(),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewLicenseHandlersClient(h.Grpc.Client)

	dataLic, err := rClient.AddLicense(ctx, dataLicense)
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}

	licenseKey := base64.StdEncoding.EncodeToString(dataLic.License)

	return httputil.StatusOK(c, "License key", licenseKey)
}
