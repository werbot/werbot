package websocket

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/contrib/websocket"
	"github.com/werbot/werbot/internal/version"
)

func (h *Handler) handleInfo(c *websocket.Conn) {
	versions := make(map[string]string)
	versions["ui"] = fmt.Sprintf("%s (%s)", "2", version.Commit())
	versions["api"] = fmt.Sprintf("%s (%s)", version.Version(), version.Commit())

	data, _ := json.Marshal(versions)
	if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
		return
	}
}
