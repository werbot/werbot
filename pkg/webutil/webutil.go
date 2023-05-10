package webutil

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HTTPResponse represents response body of API
type HTTPResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Result  any    `json:"result,omitempty"`
}

// Response is a takes in a Fiber context object, an HTTP status code, a message string and some data.
func Response(c *fiber.Ctx, status int, message string, data any) error {
	var success bool
	if status == fiber.StatusOK {
		success = true
	}
	if len(message) > 0 {
		return c.Status(status).JSON(HTTPResponse{
			Success: success,
			Message: message,
			Result:  data,
		})
	}

	return c.Status(status).JSON(data)
}

// FromGRPC is converts gRPC errors into HTTP errors and returns an error response.
func FromGRPC(c *fiber.Ctx, err error, messages ...any) error {
	statusCode := fiber.StatusOK
	dataError := status.Convert(err)

	switch dataError.Code() {
	case codes.AlreadyExists, codes.Aborted, codes.InvalidArgument, codes.Unknown: // 400 error
		statusCode = fiber.StatusBadRequest
	case codes.PermissionDenied, codes.Unauthenticated: // 401 error
		statusCode = fiber.StatusUnauthorized
	case codes.NotFound: // 404 error
		statusCode = fiber.StatusNotFound
	case codes.Internal: // 500 error
		statusCode = fiber.StatusInternalServerError
	}

	var desc any
	if len(messages) == 1 {
		desc = messages[0]
	} else if len(messages) > 1 {
		desc = messages
	} else {
		desc = dataError.Message()
	}

	// h.log.Error(err).Send()

	if statusCode != fiber.StatusOK {
		return Response(c, statusCode, utils.StatusMessage(statusCode), desc)
	}

	return nil
}

// StatusOK - HTTP error code 400
func StatusOK(c *fiber.Ctx, message string, data any) error {
	return Response(c, 200, message, data)
}
