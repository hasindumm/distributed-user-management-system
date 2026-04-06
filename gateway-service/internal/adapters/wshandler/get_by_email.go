package wshandler

import (
	"context"
	"encoding/json"
	"log/slog"

	"gateway-service/internal/ports"
)

type getByEmailHandler struct {
	svc    ports.UserService
	logger *slog.Logger
}

func (h *getByEmailHandler) Handle(c *Client, msg Message) {
	var payload struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil || payload.Email == "" {
		c.send <- errorResponse(msg, "BAD_REQUEST", "email is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	user, err := h.svc.GetUserByEmail(ctx, payload.Email)
	if err != nil {
		c.send <- mapServiceError(msg, err)
		return
	}

	c.send <- Response{Action: msg.Action, RequestID: msg.RequestID, Success: true, Payload: user}
}
