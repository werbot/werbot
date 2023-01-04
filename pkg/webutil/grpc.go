package webutil

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"google.golang.org/grpc"

	"github.com/werbot/werbot/pkg/logger"
)

// FromGRPC is ...
func FromGRPC(c *fiber.Ctx, log logger.Logger, err error) error {
	errMessage := grpc.ErrorDesc(err)

	notFound := strings.ToLower(utils.StatusMessage(404))
	if errMessage == notFound {
		return StatusNotFound(c, notFound, nil)
	}

	alreadyExists := "object already exists"
	if errMessage == alreadyExists {
		return StatusBadRequest(c, alreadyExists, nil)
	}

	log.FromGRPC(err, 2).Send()
	return InternalServerError(c, strings.ToLower(utils.StatusMessage(500)), nil)
}
