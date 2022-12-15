//go:build !saas

package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/storage/cache"
)

func handler(h *web.Handler) {}
