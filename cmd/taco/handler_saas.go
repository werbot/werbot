//go:build saas

package main

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/cache"
	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/web/module/subscription"
)

func handler(app *fiber.App, grpc *grpc.ClientService, cache cache.Cache) {
	subscription.NewHandler(app, grpc, cache).Routes()
}
