package websocket

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/redis/go-redis/v9"
)

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
