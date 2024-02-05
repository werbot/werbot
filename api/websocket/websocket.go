package websocket

import (
	"context"
	"fmt"

	"github.com/gofiber/contrib/websocket"
	"github.com/redis/go-redis/v9"
)

// sendInRedis publish messages to Redis
func (h *Handler) sendToChannel(ctx context.Context, channel, message string) error {
	if err := h.Redis.Client.Publish(ctx, channel, message).Err(); err != nil {
		return fmt.Errorf("error publishing message to channel %s: %w", channel, err)
	}
	return nil
}

func (h *Handler) publishToWebSocket(c *websocket.Conn, ch <-chan *redis.Message) {
	for msg := range ch {
		if err := c.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
			h.log.Error(err).Msg("write")
			break
		}
	}
}

func (h *Handler) processRequest(c *websocket.Conn, request *Request) {
	switch request.Action {
	case "info":
		h.handleInfo(c)
	default:
		h.sendResponse(c, 400, "error", "Not an action")
	}
}

func (h *Handler) sendResponse(c *websocket.Conn, code int, action string, data any) {
	response := Response{Code: code, Action: action, Data: data}
	if err := c.WriteJSON(response); err != nil {
		h.log.Error(err).Msg("Failed to write response")
	}
}
