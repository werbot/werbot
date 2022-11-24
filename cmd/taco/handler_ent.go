//go:build !saas

package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/werbot/werbot/internal/storage/cache"
	"github.com/werbot/werbot/internal/grpc"
)

func handler(app *fiber.App, grpc *grpc.ClientService, cache cache.Cache) {}
