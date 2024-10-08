package webutil

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/werbot/werbot/pkg/utils/errutil"
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
		codes.AlreadyExists:      fiber.StatusConflict,
		codes.PermissionDenied:   fiber.StatusForbidden,
		codes.ResourceExhausted:  fiber.StatusTooManyRequests,
		codes.FailedPrecondition: fiber.StatusBadRequest,
		codes.Aborted:            fiber.StatusConflict,
		codes.OutOfRange:         fiber.StatusBadRequest,
		codes.Unimplemented:      fiber.StatusNotImplemented,
		codes.Internal:           fiber.StatusInternalServerError,
		codes.Unavailable:        fiber.StatusServiceUnavailable,
		codes.DataLoss:           fiber.StatusInternalServerError,
		codes.Unauthenticated:    fiber.StatusUnauthorized,
	}

	if err == nil {
		return fiber.StatusOK
	}

	dataError := status.Convert(err)
	if statusCode, ok := codeMap[dataError.Code()]; ok {
		return statusCode
	}
	return fiber.StatusInternalServerError
}

// FromGRPC is converts gRPC errors into HTTP errors and returns an error response.
func FromGRPC(c *fiber.Ctx, err error) error {
	statusCode := CodeFromErrorGRPC(err)
	statusMessage := utils.StatusMessage(statusCode)

	if statusCode == fiber.StatusOK {
		return nil
	}

	var dataError any
	switch statusCode {
	case fiber.StatusInternalServerError:
		dataError = nil
	case fiber.StatusBadRequest:
		msg := status.Convert(err).Message()
		if strings.Contains(msg, ":") {
			dataError = errutil.StringToErrorMap(msg)
		} else {
			dataError = msg
		}
	default:
		dataError = status.Convert(err).Message()
	}

	return Response(c, statusCode, statusMessage, dataError)
}

// StatusOK is ...
// 200 OK
func StatusOK(c *fiber.Ctx, message string, data any) error {
	return Response(c, fiber.StatusOK, message, data)
}

// StatusBadRequest is ...
// 400 Bad Request
func StatusBadRequest(c *fiber.Ctx, data any) error {
	return Response(c, fiber.StatusBadRequest, utils.StatusMessage(fiber.StatusBadRequest), data)
}

// StatusUnauthorized is ...
// 401 Unauthorized
func StatusUnauthorized(c *fiber.Ctx, data any) error {
	return Response(c, fiber.StatusUnauthorized, utils.StatusMessage(fiber.StatusUnauthorized), data)
}

// StatusForbidden is ...
// 403 Forbidden
func StatusForbidden(c *fiber.Ctx, data any) error {
	return Response(c, fiber.StatusForbidden, utils.StatusMessage(fiber.StatusForbidden), data)
}

// StatusNotFound is ...
// 404 Not Found
func StatusNotFound(c *fiber.Ctx, data any) error {
	return Response(c, fiber.StatusNotFound, utils.StatusMessage(fiber.StatusNotFound), data)
}

// StatusInternalServerError is ...
// 500 Internal Server Error
func StatusInternalServerError(c *fiber.Ctx, data any) error {
	return Response(c, fiber.StatusInternalServerError, utils.StatusMessage(fiber.StatusInternalServerError), data)
}
