package websocket

import (
	"context"
	"encoding/json"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler manages WebSocket connections and messaging.
type Handler struct {
	*api.Handler
	log logger.Logger
}

// Request represents the structure of incoming WebSocket messages.
type Request struct {
	Action string `json:"action"`
	Token  string `json:"token"`
}

// Response represents the structure of outgoing WebSocket messages.
type Response struct {
	Code   int    `json:"code"`
	Action string `json:"action"`
	Data   any    `json:"data,omitempty"`
}

// New creates a new instance of Handler with the provided api.Handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes sets up the WebSocket routes for the application.
func (h *Handler) Routes() {
	// Middleware to upgrade HTTP requests to WebSocket protocol
	h.App.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// WebSocket endpoint
	h.App.Get("/ws/:session_id<guid>", websocket.New(func(c *websocket.Conn) {
		defer c.Close()

		ctx := context.Background()
		h.sendResponse(c, 200, "subscribe", nil)

		// c.Params("session_id")
		profileID, err := h.Redis.Client.HGet(ctx, "refresh_token:"+c.Params("session_id"), "profile_id").Result()
		if err != nil {
			h.sendResponse(c, 401, "error", "Token is not valid")
		}

		pubsub := h.Redis.Client.Subscribe(ctx, "ws:"+profileID)
		defer pubsub.Close()
		ch := pubsub.Channel()
		go h.publishToWebSocket(c, ch)

		for {
			mt, msg, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					h.log.Error(err).Msgf("Error reading message")
				}
				break
			}

			if mt == websocket.TextMessage {
				var request Request
				if err := json.Unmarshal(msg, &request); err != nil {
					// break
					continue
				}

				h.processRequest(c, &request)
			}
		}

		if err := pubsub.Unsubscribe(ctx); err != nil {
			h.log.Error(err).Msg("Error unsubscribing")
		}
	}))
}
