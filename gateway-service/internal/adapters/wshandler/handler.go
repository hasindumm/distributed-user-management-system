package wshandler

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"

	"gateway-service/internal/ports"
)

type ActionHandler interface {
	Handle(c *Client, msg Message)
}

type ActionRouter struct {
	handlers map[string]ActionHandler
	logger   *slog.Logger
}

func NewActionRouter(
	svc ports.UserService,
	client ports.UserClient,
	hub *Hub,
	logger *slog.Logger,
) *ActionRouter {
	return &ActionRouter{
		handlers: map[string]ActionHandler{
			"user.create":       &createHandler{svc: svc, logger: logger},
			"user.get_by_id":    &getByIDHandler{svc: svc, logger: logger},
			"user.get_by_email": &getByEmailHandler{svc: svc, logger: logger},
			"user.list":         &listHandler{svc: svc, logger: logger},
			"user.update":       &updateHandler{svc: svc, logger: logger},
			"user.delete":       &deleteHandler{svc: svc, logger: logger},
			"user.subscribe":    &subscribeHandler{client: client, hub: hub, logger: logger},
			"user.unsubscribe":  &unsubscribeHandler{logger: logger},
		},
		logger: logger,
	}
}

func (r *ActionRouter) Route(c *Client, msg Message) {
	handler, ok := r.handlers[msg.Action]
	if !ok {
		c.send <- Response{
			Action:    msg.Action,
			RequestID: msg.RequestID,
			Success:   false,
			Error: &WSError{
				Code:    "UNKNOWN_ACTION",
				Message: "unknown action: " + msg.Action,
			},
		}
		r.logger.Warn("ws: unknown action received", "action", msg.Action)
		return
	}
	r.logger.Info("ws: routing action", "action", msg.Action,
		"request_id", msg.RequestID)
	handler.Handle(c, msg)
}

type WSHandler struct {
	hub      *Hub
	router   *ActionRouter
	upgrader websocket.Upgrader
	cfg      Config
	logger   *slog.Logger
}

func NewWSHandler(
	svc ports.UserService,
	client ports.UserClient,
	hub *Hub,
	cfg Config,
	logger *slog.Logger,
) *WSHandler {
	router := NewActionRouter(svc, client, hub, logger)
	return &WSHandler{
		hub:    hub,
		router: router,
		cfg:    cfg,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// TODO: restrict origins in production
				return true
			},
		},
		logger: logger,
	}
}

func New(svc ports.UserService, client ports.UserClient, logger *slog.Logger) *WSHandler {
	hub := NewHub(logger)
	go hub.Run()
	return NewWSHandler(svc, client, hub, DefaultConfig(), logger)
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("ws: upgrade failed", "error", err)
		return
	}

	client := newClient(h.hub, conn, h.cfg, h.logger)
	h.hub.register <- client

	go client.writePump()

	client.readPump(h.router)
}
