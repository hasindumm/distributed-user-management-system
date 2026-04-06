package wshandler

import (
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	hub           *Hub
	conn          *websocket.Conn
	send          chan Response
	cfg           Config
	mu            sync.RWMutex
	subscriptions map[string]bool
	logger        *slog.Logger
}

func newClient(hub *Hub, conn *websocket.Conn, cfg Config, logger *slog.Logger) *Client {
	return &Client{
		hub:           hub,
		conn:          conn,
		send:          make(chan Response, cfg.SendBufferSize),
		cfg:           cfg,
		subscriptions: make(map[string]bool),
		logger:        logger,
	}
}

func (c *Client) subscribe(eventType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscriptions[eventType] = true
}

func (c *Client) unsubscribe(eventType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subscriptions, eventType)
}

func (c *Client) isSubscribed(eventType string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.subscriptions[eventType]
}

func (c *Client) writePump() {
	ticker := time.NewTicker(c.cfg.PingPeriod)
	defer func() {
		ticker.Stop()
		if err := c.conn.Close(); err != nil {
			c.logger.Error("ws: failed to close connection in writePump", "error", err)
		}
	}()

	for {
		select {
		case resp, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteWait)); err != nil {
				c.logger.Error("ws: failed to set write deadline", "error", err)
				return
			}
			if !ok {
				if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					c.logger.Error("ws: failed to write close message", "error", err)
				}
				return
			}
			if err := c.conn.WriteJSON(resp); err != nil {
				c.logger.Error("ws: failed to write message", "error", err)
				return
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteWait)); err != nil {
				c.logger.Error("ws: failed to set write deadline for ping", "error", err)
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logger.Warn("ws: ping failed, closing connection", "error", err)
				return
			}
		}
	}
}

func (c *Client) readPump(router *ActionRouter) {
	defer func() {
		c.hub.unregister <- c
		if err := c.conn.Close(); err != nil {
			c.logger.Error("ws: failed to close connection in readPump", "error", err)
		}
	}()

	c.conn.SetReadLimit(c.cfg.MaxMessageSize)

	if err := c.conn.SetReadDeadline(time.Now().Add(c.cfg.PongWait)); err != nil {
		c.logger.Error("ws: failed to set initial read deadline", "error", err)
		return
	}

	c.conn.SetPongHandler(func(string) error {
		if err := c.conn.SetReadDeadline(time.Now().Add(c.cfg.PongWait)); err != nil {
			c.logger.Error("ws: failed to reset read deadline on pong", "error", err)
			return err
		}
		return nil
	})

	for {
		var msg Message
		if err := c.conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure) {
				c.logger.Warn("ws: connection closed unexpectedly", "error", err)
			}
			break
		}
		router.Route(c, msg)
	}
}
