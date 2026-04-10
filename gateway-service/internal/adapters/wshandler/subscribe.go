package wshandler

import (
	"log/slog"

	"gateway-service/internal/ports"
	"user-service/pkg/userclient"
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

	// create event channel
	eventCh := make(chan userclient.Event, sendBufferSize)

	_, err := h.client.Subscribe(eventCh)
	if err != nil {
		c.send <- errorResponse(msg, "INTERNAL_ERROR", "failed to subscribe to events")
		return
	}

	go func() {
		for evt := range eventCh {
			switch evt.Type {
			case created:
				e, ok := evt.Payload.(userclient.UserCreatedEvent)
				if !ok {
					h.logger.Error("ws: unexpected payload type", "event_type", evt.Type)
					continue
				}
				h.hub.Broadcast("user.created", Response{
					Action:  "user.created",
					Success: true,
					Payload: toUserResponse(e.User),
				})
			case updated:
				e, ok := evt.Payload.(userclient.UserUpdatedEvent)
				if !ok {
					h.logger.Error("ws: unexpected payload type", "event_type", evt.Type)
					continue
				}
				h.hub.Broadcast("user.updated", Response{
					Action:  "user.updated",
					Success: true,
					Payload: toUserResponse(e.User),
				})
			case deleted:
				e, ok := evt.Payload.(userclient.UserDeletedEvent)
				if !ok {
					h.logger.Error("ws: unexpected payload type", "event_type", evt.Type)
					continue
				}
				h.hub.Broadcast("user.deleted", Response{
					Action:  "user.deleted",
					Success: true,
					Payload: map[string]string{"user_id": e.UserID},
				})
			}
		}
	}()

	c.send <- Response{Action: msg.Action, RequestID: msg.RequestID, Success: true}
	h.logger.Info("ws: client subscribed to user events")
}
