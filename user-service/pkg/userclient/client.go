package userclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

type Config struct {
	NATSURL      string
	Timeout      time.Duration
	CacheEnabled bool
}

type cacheUpdate struct {
	eventType string
	user      *UserDTO
	userID    string
}

type Client struct {
	nc               *nats.Conn
	timeout          time.Duration
	logger           *slog.Logger
	cache            *cache
	cacheSub         *Subscription
	updateCh         chan cacheUpdate
	stopCh           chan struct{}
	cacheFullyLoaded bool
}

func New(cfg Config, logger *slog.Logger) (*Client, error) {
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return nil, fmt.Errorf("userclient: failed to connect to NATS: %w", err)
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	logger.Info("user client connected to NATS", "url", cfg.NATSURL)

	c := &Client{
		nc:      nc,
		timeout: timeout,
		logger:  logger,
	}

	if cfg.CacheEnabled {
		c.cache = newCache()
		c.updateCh = make(chan cacheUpdate, cacheUpdateBufferSize)
		c.stopCh = make(chan struct{})
		go c.processCacheUpdates()
		logger.Info("userclient: cache enabled")
		c.loadCache()
		c.subscribeForCacheSync()
	}

	return c, nil
}

func (c *Client) loadCache() {
	ctx, cancel := context.WithTimeout(context.Background(), cacheStartupTimeout)
	defer cancel()

	offset := 0
	total := 0

	for {
		req := ListUsersRequest{
			Limit:  cacheStartupBatchSize,
			Offset: int32(offset),
		}
		var resp ListUsersResponse
		if err := c.request(ctx, SubjectListUsers, req, &resp); err != nil {
			c.logger.Warn("userclient: cache startup load failed",
				"offset", offset,
				"loaded_so_far", total,
				"error", err)
			return
		}
		if resp.Error != nil {
			c.logger.Warn("userclient: cache startup load returned error",
				"offset", offset,
				"error", resp.Error.Message)
			return
		}

		for _, u := range resp.Users {
			c.cache.set(u)
		}
		total += len(resp.Users)

		c.logger.Info("userclient: cache startup batch loaded",
			"batch_size", len(resp.Users),
			"total_loaded", total,
			"offset", offset)

		if len(resp.Users) < int(cacheStartupBatchSize) {
			c.cacheFullyLoaded = true
			break
		}

		offset += int(cacheStartupBatchSize)
	}

	c.logger.Info("userclient: cache loaded at startup",
		"total_users", total,
		"fully_loaded", c.cacheFullyLoaded)
}

func (c *Client) subscribeForCacheSync() {
	sub, err := c.Subscribe(EventHandlers{
		OnCreated: func(evt UserCreatedEvent) {
			select {
			case c.updateCh <- cacheUpdate{eventType: "created", user: &evt.User}:
			default:
				c.logger.Warn("userclient: cache update channel full, dropping created event",
					"user_id", evt.User.UserID)
			}
		},
		OnUpdated: func(evt UserUpdatedEvent) {
			select {
			case c.updateCh <- cacheUpdate{eventType: "updated", user: &evt.User}:
			default:
				c.logger.Warn("userclient: cache update channel full, dropping updated event",
					"user_id", evt.User.UserID)
			}
		},
		OnDeleted: func(evt UserDeletedEvent) {
			select {
			case c.updateCh <- cacheUpdate{eventType: "deleted", userID: evt.UserID}:
			default:
				c.logger.Warn("userclient: cache update channel full, dropping deleted event",
					"user_id", evt.UserID)
			}
		},
	})
	if err != nil {
		c.logger.Warn("userclient: failed to subscribe for cache sync, cache may become stale", "error", err)
		return
	}
	c.cacheSub = sub

	if err := c.nc.Flush(); err != nil {
		c.logger.Warn("userclient: failed to flush NATS connection after subscribing for cache sync",
			"error", err)
	}

	c.logger.Info("userclient: cache sync subscription active")
}

func (c *Client) processCacheUpdates() {
	for {
		select {
		case update, ok := <-c.updateCh:
			if !ok {
				// channel was closed — drain complete
				return
			}
			c.applyCacheUpdate(update)

		case <-c.stopCh:
			for {
				select {
				case update := <-c.updateCh:
					c.applyCacheUpdate(update)
				default:
					c.logger.Info("userclient: cache update processor stopped")
					return
				}
			}
		}
	}
}

func (c *Client) applyCacheUpdate(update cacheUpdate) {
	switch update.eventType {
	case "created", "updated":
		if update.user == nil {
			c.logger.Error("userclient: cache update has nil user", "event_type", update.eventType)
			return
		}
		c.cache.set(*update.user)
		c.logger.Info("userclient: cache updated via event",
			"event_type", update.eventType,
			"user_id", update.user.UserID)
	case "deleted":
		c.cache.delete(update.userID)
		c.logger.Info("userclient: cache entry removed via event",
			"user_id", update.userID)
	default:
		c.logger.Error("userclient: unknown cache update event type",
			"event_type", update.eventType)
	}
}

func NewWithConn(nc *nats.Conn, timeout time.Duration, logger *slog.Logger) *Client {
	if timeout == 0 {
		timeout = defaultTimeout
	}
	return &Client{nc: nc, timeout: timeout, logger: logger}
}

func (c *Client) Close() {
	if c.cacheSub != nil {
		if err := c.cacheSub.Unsubscribe(); err != nil {
			c.logger.Error("userclient: error unsubscribing cache sync", "error", err)
		}
	}

	if c.stopCh != nil {
		close(c.stopCh)
	}

	if err := c.nc.Drain(); err != nil {
		c.logger.Error("userclient: error draining NATS connection", "error", err)
	}
}

func (c *Client) CreateUser(ctx context.Context, req CreateUserRequest) (UserDTO, error) {
	var resp CreateUserResponse
	if err := c.request(ctx, SubjectCreateUser, req, &resp); err != nil {
		return UserDTO{}, err
	}
	if resp.Error != nil {
		return UserDTO{}, mapRPCError(resp.Error)
	}
	return *resp.User, nil
}

func (c *Client) GetUserByID(ctx context.Context, userID string) (UserDTO, error) {
	if c.cache != nil {
		if u, ok := c.cache.get(userID); ok {
			c.logger.DebugContext(ctx, "userclient: cache hit", "user_id", userID)
			return u, nil
		}
		c.logger.DebugContext(ctx, "userclient: cache miss, falling through to RPC", "user_id", userID)
	}

	req := GetUserByIDRequest{UserID: userID}
	var resp GetUserByIDResponse
	if err := c.request(ctx, SubjectGetUserByID, req, &resp); err != nil {
		return UserDTO{}, err
	}
	if resp.Error != nil {
		return UserDTO{}, mapRPCError(resp.Error)
	}

	if c.cache != nil {
		c.cache.set(*resp.User)
		c.logger.DebugContext(ctx, "userclient: cache populated on miss", "user_id", userID)
	}

	return *resp.User, nil
}

func (c *Client) GetUserByEmail(ctx context.Context, email string) (UserDTO, error) {
	if c.cache != nil {
		if u, ok := c.cache.getByEmail(email); ok {
			c.logger.DebugContext(ctx, "userclient: cache hit", "email", email)
			return u, nil
		}
		c.logger.DebugContext(ctx, "userclient: cache miss, falling through to RPC", "email", email)
	}

	req := GetUserByEmailRequest{Email: email}
	var resp GetUserByEmailResponse
	if err := c.request(ctx, SubjectGetUserByEmail, req, &resp); err != nil {
		return UserDTO{}, err
	}
	if resp.Error != nil {
		return UserDTO{}, mapRPCError(resp.Error)
	}

	if c.cache != nil {
		c.cache.set(*resp.User)
		c.logger.DebugContext(ctx, "userclient: cache populated on email miss", "email", email)
	}

	return *resp.User, nil
}

func (c *Client) ListUsers(ctx context.Context, req ListUsersRequest) ([]UserDTO, error) {
	if req.Limit == 0 {
		req.Limit = defaultListLimit
	}
	if c.cache != nil && c.cacheFullyLoaded {
		users := c.cache.list(req.Status, req.Limit, req.Offset)
		c.logger.DebugContext(ctx, "userclient: cache hit for list",
			"count", len(users),
			"limit", req.Limit,
			"offset", req.Offset)
		return users, nil
	}

	if c.cache != nil && !c.cacheFullyLoaded {
		c.logger.DebugContext(ctx,
			"userclient: cache incomplete, falling through to RPC for list")
	}

	var resp ListUsersResponse
	if err := c.request(ctx, SubjectListUsers, req, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, mapRPCError(resp.Error)
	}
	return resp.Users, nil
}

func (c *Client) UpdateUser(ctx context.Context, req UpdateUserRequest) (UserDTO, error) {
	var resp UpdateUserResponse
	if err := c.request(ctx, SubjectUpdateUser, req, &resp); err != nil {
		return UserDTO{}, err
	}
	if resp.Error != nil {
		return UserDTO{}, mapRPCError(resp.Error)
	}
	return *resp.User, nil
}

func (c *Client) DeleteUser(ctx context.Context, userID string) error {
	req := DeleteUserRequest{UserID: userID}
	var resp DeleteUserResponse
	if err := c.request(ctx, SubjectDeleteUser, req, &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return mapRPCError(resp.Error)
	}
	return nil
}

type EventHandlers struct {
	OnCreated func(UserCreatedEvent)
	OnUpdated func(UserUpdatedEvent)
	OnDeleted func(UserDeletedEvent)
}

type Subscription struct {
	subs   []*nats.Subscription
	logger *slog.Logger
}

func (s *Subscription) Unsubscribe() error {
	for _, sub := range s.subs {
		if err := sub.Unsubscribe(); err != nil {
			s.logger.Error("userclient: failed to unsubscribe", "subject", sub.Subject, "error", err)
			return fmt.Errorf("userclient: failed to unsubscribe from %s: %w", sub.Subject, err)
		}
		s.logger.Info("userclient: unsubscribed from event", "subject", sub.Subject)
	}
	return nil
}

func (c *Client) Subscribe(handlers EventHandlers) (*Subscription, error) {
	sub := &Subscription{logger: c.logger}

	if handlers.OnCreated != nil {
		s, err := c.nc.Subscribe(SubjectUserCreated, func(msg *nats.Msg) {
			var evt UserCreatedEvent
			if err := json.Unmarshal(msg.Data, &evt); err != nil {
				c.logger.Error("userclient: failed to unmarshal created event", "error", err)
				return
			}
			c.logger.Debug("userclient: received user created event", "user_id", evt.User.UserID)
			handlers.OnCreated(evt)
		})
		if err != nil {
			return nil, fmt.Errorf("userclient: failed to subscribe to %s: %w", SubjectUserCreated, err)
		}
		sub.subs = append(sub.subs, s)
	}

	if handlers.OnUpdated != nil {
		s, err := c.nc.Subscribe(SubjectUserUpdated, func(msg *nats.Msg) {
			var evt UserUpdatedEvent
			if err := json.Unmarshal(msg.Data, &evt); err != nil {
				c.logger.Error("userclient: failed to unmarshal updated event", "error", err)
				return
			}
			c.logger.Debug("userclient: received user updated event", "user_id", evt.User.UserID)
			handlers.OnUpdated(evt)
		})
		if err != nil {
			return nil, fmt.Errorf("userclient: failed to subscribe to %s: %w", SubjectUserUpdated, err)
		}
		sub.subs = append(sub.subs, s)
	}

	if handlers.OnDeleted != nil {
		s, err := c.nc.Subscribe(SubjectUserDeleted, func(msg *nats.Msg) {
			var evt UserDeletedEvent
			if err := json.Unmarshal(msg.Data, &evt); err != nil {
				c.logger.Error("userclient: failed to unmarshal deleted event", "error", err)
				return
			}
			c.logger.Debug("userclient: received user deleted event", "user_id", evt.UserID)
			handlers.OnDeleted(evt)
		})
		if err != nil {
			return nil, fmt.Errorf("userclient: failed to subscribe to %s: %w", SubjectUserDeleted, err)
		}
		sub.subs = append(sub.subs, s)
	}

	c.logger.Info("userclient: subscribed to user events")
	return sub, nil
}

func (c *Client) request(ctx context.Context, subject string, payload, out any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("userclient: failed to marshal request for %s: %w", subject, err)
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	c.logger.DebugContext(ctx, "sending RPC request", "subject", subject)

	msg, err := c.nc.RequestWithContext(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("userclient: RPC request to %s failed: %w", subject, err)
	}

	if err := json.Unmarshal(msg.Data, out); err != nil {
		return fmt.Errorf("userclient: failed to unmarshal response from %s: %w", subject, err)
	}

	return nil
}
