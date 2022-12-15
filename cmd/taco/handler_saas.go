//go:build saas

package main

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/api/web/subscription"
	"github.com/werbot/werbot/internal/grpc"
)

func handler(app *fiber.App, grpc *grpc.ClientService, auth fiber.Handler) {
	subscription.New(app, grpc, auth).Routes()
}
