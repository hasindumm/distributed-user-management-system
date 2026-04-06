package wshandler

import "log/slog"

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan broadcastMessage
	logger     *slog.Logger
}

type broadcastMessage struct {
	eventType string
	response  Response
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan broadcastMessage, sendBufferSize),
		logger:     logger,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			h.logger.Info("ws: client connected",
				"total_clients", len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.logger.Info("ws: client disconnected",
					"total_clients", len(h.clients))
			}

		case msg := <-h.broadcast:
			for client := range h.clients {
				if !client.isSubscribed(msg.eventType) {
					continue
				}
				select {
				case client.send <- msg.response:
				default:
					// client send buffer full — client is too slow
					// unregister and close — do not block the Hub
					delete(h.clients, client)
					close(client.send)
					h.logger.Warn("ws: client send buffer full, disconnecting",
						"event_type", msg.eventType)
				}
			}
		}
	}
}

func (h *Hub) Broadcast(eventType string, resp Response) {
	select {
	case h.broadcast <- broadcastMessage{eventType: eventType, response: resp}:
	default:
		h.logger.Warn("ws: hub broadcast buffer full, dropping event",
			"event_type", eventType)
	}
}
