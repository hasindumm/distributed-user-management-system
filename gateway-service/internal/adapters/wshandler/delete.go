package wshandler

import (
	"context"
	"encoding/json"
	"log/slog"

	"gateway-service/internal/ports"
)

type deleteHandler struct {
	svc    ports.UserService
	logger *slog.Logger
}

func (h *deleteHandler) Handle(c *Client, msg Message) {
	var payload struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil || payload.UserID == "" {
		c.send <- errorResponse(msg, "BAD_REQUEST", "user_id is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	if err := h.svc.DeleteUser(ctx, payload.UserID); err != nil {
		c.send <- mapServiceError(msg, err)
		return
	}

	c.send <- Response{Action: msg.Action, RequestID: msg.RequestID, Success: true}
}
