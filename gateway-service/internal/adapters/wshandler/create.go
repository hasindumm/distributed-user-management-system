package wshandler

import (
	"context"
	"encoding/json"
	"log/slog"

	"gateway-service/internal/dto"
	"gateway-service/internal/ports"
)

type createHandler struct {
	svc    ports.UserService
	logger *slog.Logger
}

func (h *createHandler) Handle(c *Client, msg Message) {
	var req dto.CreateUserRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.send <- errorResponse(msg, "BAD_REQUEST", "invalid payload")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	user, err := h.svc.CreateUser(ctx, req)
	if err != nil {
		c.send <- mapServiceError(msg, err)
		return
	}

	c.send <- Response{Action: msg.Action, RequestID: msg.RequestID, Success: true, Payload: user}
}
