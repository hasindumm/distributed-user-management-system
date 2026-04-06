package wshandler

import (
	"log/slog"

	"gateway-service/internal/ports"
)

type subscribeHandler struct {
	client ports.UserClient
	hub    *Hub
	logger *slog.Logger
}

func (h *subscribeHandler) Handle(c *Client, msg Message) {
	eventTypes := []string{"user.created", "user.updated", "user.deleted"}
	for _, et := range eventTypes {
		c.subscribe(et)
	}

	_, err := h.client.Subscribe(ports.EventHandlers{
		OnCreated: func(evt ports.UserCreatedEvent) {
			h.hub.Broadcast("user.created", Response{
				Action:  "user.created",
				Success: true,
				Payload: evt.User,
			})
		},
		OnUpdated: func(evt ports.UserUpdatedEvent) {
			h.hub.Broadcast("user.updated", Response{
				Action:  "user.updated",
				Success: true,
				Payload: evt.User,
			})
		},
		OnDeleted: func(evt ports.UserDeletedEvent) {
			h.hub.Broadcast("user.deleted", Response{
				Action:  "user.deleted",
				Success: true,
				Payload: map[string]string{"user_id": evt.UserID},
			})
		},
	})
	if err != nil {
		c.send <- errorResponse(msg, "INTERNAL_ERROR", "failed to subscribe to events")
		return
	}

	c.send <- Response{Action: msg.Action, RequestID: msg.RequestID, Success: true}
	h.logger.Info("ws: client subscribed to user events")
}
