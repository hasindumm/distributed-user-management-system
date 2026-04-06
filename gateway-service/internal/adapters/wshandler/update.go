package wshandler

import (
	"context"
	"encoding/json"
	"log/slog"

	"gateway-service/internal/dto"
	"gateway-service/internal/ports"
)

type updateHandler struct {
	svc    ports.UserService
	logger *slog.Logger
}

func (h *updateHandler) Handle(c *Client, msg Message) {
	var payload struct {
		UserID string `json:"user_id"`
		dto.UpdateUserRequest
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.send <- errorResponse(msg, "BAD_REQUEST", "invalid payload")
		return
	}
	if payload.UserID == "" {
		c.send <- errorResponse(msg, "BAD_REQUEST", "user_id is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	user, err := h.svc.UpdateUser(ctx, payload.UserID, payload.UpdateUserRequest)
	if err != nil {
		c.send <- mapServiceError(msg, err)
		return
	}

	c.send <- Response{Action: msg.Action, RequestID: msg.RequestID, Success: true, Payload: user}
}
