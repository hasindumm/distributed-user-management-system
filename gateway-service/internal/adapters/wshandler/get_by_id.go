package wshandler

import (
	"context"
	"encoding/json"
	"log/slog"

	"gateway-service/internal/ports"
)

type getByIDHandler struct {
	svc    ports.UserService
	logger *slog.Logger
}

func (h *getByIDHandler) Handle(c *Client, msg Message) {
	var payload struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil || payload.UserID == "" {
		c.send <- errorResponse(msg, "BAD_REQUEST", "user_id is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	user, err := h.svc.GetUserByID(ctx, payload.UserID)
	if err != nil {
		c.send <- mapServiceError(msg, err)
		return
	}

	c.send <- Response{Action: msg.Action, RequestID: msg.RequestID, Success: true, Payload: user}
}
