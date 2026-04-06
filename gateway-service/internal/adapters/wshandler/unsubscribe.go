package wshandler

import (
	"log/slog"
)

type unsubscribeHandler struct {
	logger *slog.Logger
}

func (h *unsubscribeHandler) Handle(c *Client, msg Message) {
	eventTypes := []string{"user.created", "user.updated", "user.deleted"}
	for _, et := range eventTypes {
		c.unsubscribe(et)
	}

	c.send <- Response{Action: msg.Action, RequestID: msg.RequestID, Success: true}
	h.logger.Info("ws: client unsubscribed from user events")
}
