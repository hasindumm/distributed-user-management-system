package wshandler

import (
	"context"
	"encoding/json"
	"log/slog"

	"gateway-service/internal/dto"
	"gateway-service/internal/ports"
)

type listHandler struct {
	svc    ports.UserService
	logger *slog.Logger
}

func (h *listHandler) Handle(c *Client, msg Message) {
	req := dto.ListUsersRequest{Limit: 20}
	if len(msg.Payload) > 0 && string(msg.Payload) != "null" {
		if err := json.Unmarshal(msg.Payload, &req); err != nil {
			c.send <- errorResponse(msg, "BAD_REQUEST", "invalid payload")
			return
		}
		if req.Limit == 0 {
			req.Limit = 20
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	users, err := h.svc.ListUsers(ctx, req)
	if err != nil {
		c.send <- mapServiceError(msg, err)
		return
	}

	c.send <- Response{Action: msg.Action, RequestID: msg.RequestID, Success: true, Payload: users}
}
