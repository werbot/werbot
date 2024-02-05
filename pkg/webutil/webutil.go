package webutil

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HTTPResponse represents response body of API
type HTTPResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  any    `json:"result,omitempty"`
}

// Response is a takes in a Fiber context object, an HTTP status code, a message string and some data.
func Response(c *fiber.Ctx, code int, message string, data any) error {
	if message == "" {
		return c.Status(code).JSON(data)
	}

	return c.Status(code).JSON(HTTPResponse{
		Code:    code,
		Message: message,
		Result:  data,
	})
}

// CodeFromErrorGRPC is ...
func CodeFromErrorGRPC(err error) int {
	codeMap := map[codes.Code]int{
		codes.OK:                 fiber.StatusOK,
		codes.Canceled:           fiber.StatusBadRequest,
		codes.Unknown:            fiber.StatusInternalServerError,
		codes.InvalidArgument:    fiber.StatusBadRequest,
		codes.DeadlineExceeded:   fiber.StatusGatewayTimeout,
		codes.NotFound:           fiber.StatusNotFound,
		codes.AlreadyExists:      fiber.StatusBadRequest,
		codes.PermissionDenied:   fiber.StatusForbidden,
		codes.ResourceExhausted:  fiber.StatusTooManyRequests,
		codes.FailedPrecondition: fiber.StatusBadRequest,
		codes.Aborted:            fiber.StatusBadRequest,
		codes.OutOfRange:         fiber.StatusBadRequest,
		codes.Unimplemented:      fiber.StatusNotImplemented,
		codes.Internal:           fiber.StatusInternalServerError,
		codes.Unavailable:        fiber.StatusServiceUnavailable,
		codes.DataLoss:           fiber.StatusInternalServerError,
		codes.Unauthenticated:    fiber.StatusUnauthorized,
	}

	dataError := status.Convert(err)
	statusCode, ok := codeMap[dataError.Code()]
	if !ok {
		return fiber.StatusInternalServerError
	}
	return statusCode
}

// FromGRPC is converts gRPC errors into HTTP errors and returns an error response.
func FromGRPC(c *fiber.Ctx, err error) error {
	statusCode := CodeFromErrorGRPC(err)
	if statusCode == fiber.StatusOK {
		return nil
	}

	var dataError any
	if statusCode != 500 {
		dataError = status.Convert(err).Message()
	}

	statusMessage := utils.StatusMessage(statusCode)
	return Response(c, statusCode, statusMessage, dataError)
}

// StatusOK is ...
// 200 ok
func StatusOK(c *fiber.Ctx, message string, data any) error {
	return Response(c, fiber.StatusOK, message, data)
}

// StatusBadRequest is ...
// 400 error
func StatusBadRequest(c *fiber.Ctx, data any) error {
	return Response(c, fiber.StatusBadRequest, utils.StatusMessage(fiber.StatusBadRequest), data)
}

// StatusUnauthorized is ...
// 401 error
func StatusUnauthorized(c *fiber.Ctx, data any) error {
	return Response(c, fiber.StatusUnauthorized, utils.StatusMessage(fiber.StatusUnauthorized), data)
}

// StatusNotFound is ...
// 404 error
func StatusNotFound(c *fiber.Ctx, data any) error {
	return Response(c, fiber.StatusNotFound, utils.StatusMessage(fiber.StatusNotFound), data)
}

// StatusInternalServerError is ...
// 500 error
func StatusInternalServerError(c *fiber.Ctx, data any) error {
	return Response(c, fiber.StatusInternalServerError, utils.StatusMessage(fiber.StatusInternalServerError), data)
}
