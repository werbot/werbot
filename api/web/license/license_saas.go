//go:build saas

package license

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"

	pb "github.com/werbot/werbot/api/proto/license"
)

// @Summary      License expired info
// @Tags         license
// @Accept       json
// @Produce      json
// @Param        req         body     licenseInput
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/license/expired [get]
func (h *Handler) getLicenseExpired(c *fiber.Ctx) error {
	request := new(pb.License_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.License_RequestMultiError) {
			e := err.(pb.License_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	licenseDec, err := base64.StdEncoding.DecodeString(request.License)
	if err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgBadRequest, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewLicenseHandlersClient(h.Grpc.Client)
	expiredLic, err := rClient.LicenseExpired(ctx, &pb.LicenseExpired_Request{
		License: licenseDec,
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgLicenseExpired, expiredLic.Status)
}

// @Summary      Creating a new license
// @Tags         license
// @Accept       json
// @Produce      json
// @Param        req     body     pb.AddLicenseRequest
// @Success      200     {object} webutil.HTTPResponse
// @Failure      400,500 {object} webutil.HTTPResponse
// @Router       /v1/license [post]
func (h *Handler) postLicense(c *fiber.Ctx) error {
	request := new(pb.AddLicense_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.AddLicense_RequestMultiError) {
			e := err.(pb.AddLicense_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	request.Ip = request.GetIp()
	request.Token = request.GetToken()

	userParameter := middleware.AuthUser(c)
	if userParameter.IsUserAdmin() {
		request.Customer = request.GetCustomer()
		request.Subscriber = request.GetSubscriber()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewLicenseHandlersClient(h.Grpc.Client)
	dataLic, err := rClient.AddLicense(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	licenseKey := base64.StdEncoding.EncodeToString(dataLic.License)

	return webutil.StatusOK(c, msgLicenseKey, licenseKey)
}
