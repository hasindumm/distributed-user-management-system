package natsadaptor

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"
	"user-service/internal/domain"
	"user-service/internal/ports"
	"user-service/pkg/userclient"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

const handlerTimeout = 30 * time.Second

type Handler struct {
	svc      ports.UserService
	nc       *nats.Conn
	validate *validator.Validate
	logger   *slog.Logger
	subs     []*nats.Subscription
}

func NewHandler(svc ports.UserService, nc *nats.Conn, logger *slog.Logger) *Handler {
	return &Handler{
		svc:      svc,
		nc:       nc,
		validate: validator.New(),
		logger:   logger,
	}
}

func (h *Handler) Subscribe() error {
	type entry struct {
		subject string
		fn      nats.MsgHandler
	}

	entries := []entry{
		{userclient.SubjectCreateUser, h.handleCreate},
		{userclient.SubjectGetUserByID, h.handleGetByID},
		{userclient.SubjectGetUserByEmail, h.handleGetByEmail},
		{userclient.SubjectListUsers, h.handleList},
		{userclient.SubjectUpdateUser, h.handleUpdate},
		{userclient.SubjectDeleteUser, h.handleDelete},
	}

	for _, e := range entries {
		sub, err := h.nc.Subscribe(e.subject, e.fn)
		if err != nil {
			return err
		}
		h.subs = append(h.subs, sub)
		h.logger.Info("NATS handler subscribed", "subject", e.subject)
	}

	return h.nc.Flush()
}

func (h *Handler) Stop() {
	for _, sub := range h.subs {
		if err := sub.Unsubscribe(); err != nil {
			h.logger.Error("failed to unsubscribe", "subject", sub.Subject, "error", err)
		}
	}
	h.logger.Info("NATS handler stopped")
}

func (h *Handler) handleCreate(msg *nats.Msg) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	var req userclient.CreateUserRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.reply(msg, userclient.CreateUserResponse{
			Error: rpcErr(userclient.ErrCodeValidation, "invalid request format"),
		})
		return
	}

	if err := h.validate.Struct(&req); err != nil {
		h.reply(msg, userclient.CreateUserResponse{
			Error: rpcErr(userclient.ErrCodeValidation, err.Error()),
		})
		return
	}

	status := domain.UserStatusActive
	if req.Status != "" {
		status = domain.UserStatus(req.Status)
	}

	user, err := h.svc.CreateUser(ctx, req.FirstName, req.LastName, req.Email, req.Phone, req.Age, status)
	if err != nil {
		h.reply(msg, userclient.CreateUserResponse{Error: mapDomainErr(err)})
		return
	}

	dto := toDTO(user)
	h.reply(msg, userclient.CreateUserResponse{User: &dto})
	h.publishEvent(userclient.SubjectUserCreated, userclient.UserCreatedEvent{User: dto})
}

func (h *Handler) handleGetByID(msg *nats.Msg) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	var req userclient.GetUserByIDRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.reply(msg, userclient.GetUserByIDResponse{
			Error: rpcErr(userclient.ErrCodeValidation, "invalid request format"),
		})
		return
	}

	if err := h.validate.Struct(&req); err != nil {
		h.reply(msg, userclient.GetUserByIDResponse{
			Error: rpcErr(userclient.ErrCodeValidation, err.Error()),
		})
		return
	}

	id, err := uuid.Parse(req.UserID)
	if err != nil {
		h.reply(msg, userclient.GetUserByIDResponse{
			Error: rpcErr(userclient.ErrCodeValidation, "invalid UUID"),
		})
		return
	}

	user, err := h.svc.GetUserByID(ctx, id)
	if err != nil {
		h.reply(msg, userclient.GetUserByIDResponse{Error: mapDomainErr(err)})
		return
	}

	dto := toDTO(user)
	h.reply(msg, userclient.GetUserByIDResponse{User: &dto})
}

func (h *Handler) handleGetByEmail(msg *nats.Msg) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	var req userclient.GetUserByEmailRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.reply(msg, userclient.GetUserByEmailResponse{
			Error: rpcErr(userclient.ErrCodeValidation, "invalid request format"),
		})
		return
	}

	if err := h.validate.Struct(&req); err != nil {
		h.reply(msg, userclient.GetUserByEmailResponse{
			Error: rpcErr(userclient.ErrCodeValidation, err.Error()),
		})
		return
	}

	user, err := h.svc.GetUserByEmail(ctx, req.Email)
	if err != nil {
		h.reply(msg, userclient.GetUserByEmailResponse{Error: mapDomainErr(err)})
		return
	}

	dto := toDTO(user)
	h.reply(msg, userclient.GetUserByEmailResponse{User: &dto})
}

func (h *Handler) handleList(msg *nats.Msg) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	var req userclient.ListUsersRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.reply(msg, userclient.ListUsersResponse{
			Error: rpcErr(userclient.ErrCodeValidation, "invalid request format"),
		})
		return
	}

	if err := h.validate.Struct(&req); err != nil {
		h.reply(msg, userclient.ListUsersResponse{
			Error: rpcErr(userclient.ErrCodeValidation, err.Error()),
		})
		return
	}

	var statusFilter *domain.UserStatus
	if req.Status != nil {
		s := domain.UserStatus(*req.Status)
		statusFilter = &s
	}

	users, err := h.svc.ListUsers(ctx, statusFilter, req.Limit, req.Offset)
	if err != nil {
		h.reply(msg, userclient.ListUsersResponse{Error: mapDomainErr(err)})
		return
	}

	dtos := make([]userclient.UserDTO, len(users))
	for i, u := range users {
		dtos[i] = toDTO(u)
	}
	h.reply(msg, userclient.ListUsersResponse{Users: dtos})
}

func (h *Handler) handleUpdate(msg *nats.Msg) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	var req userclient.UpdateUserRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.reply(msg, userclient.UpdateUserResponse{
			Error: rpcErr(userclient.ErrCodeValidation, "invalid request format"),
		})
		return
	}

	if err := h.validate.Struct(&req); err != nil {
		h.reply(msg, userclient.UpdateUserResponse{
			Error: rpcErr(userclient.ErrCodeValidation, err.Error()),
		})
		return
	}

	id, err := uuid.Parse(req.UserID)
	if err != nil {
		h.reply(msg, userclient.UpdateUserResponse{
			Error: rpcErr(userclient.ErrCodeValidation, "invalid UUID"),
		})
		return
	}

	user := domain.User{
		UserId:    id,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Phone:     req.Phone,
		Age:       req.Age,
		Status:    domain.UserStatus(req.Status),
	}

	updated, err := h.svc.UpdateUser(ctx, user)
	if err != nil {
		h.reply(msg, userclient.UpdateUserResponse{Error: mapDomainErr(err)})
		return
	}

	dto := toDTO(updated)
	h.reply(msg, userclient.UpdateUserResponse{User: &dto})
	h.publishEvent(userclient.SubjectUserUpdated, userclient.UserUpdatedEvent{User: dto})
}

func (h *Handler) handleDelete(msg *nats.Msg) {
	ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
	defer cancel()

	var req userclient.DeleteUserRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.reply(msg, userclient.DeleteUserResponse{
			Error: rpcErr(userclient.ErrCodeValidation, "invalid request format"),
		})
		return
	}

	if err := h.validate.Struct(&req); err != nil {
		h.reply(msg, userclient.DeleteUserResponse{
			Error: rpcErr(userclient.ErrCodeValidation, err.Error()),
		})
		return
	}

	id, err := uuid.Parse(req.UserID)
	if err != nil {
		h.reply(msg, userclient.DeleteUserResponse{
			Error: rpcErr(userclient.ErrCodeValidation, "invalid UUID"),
		})
		return
	}

	if err := h.svc.DeleteUser(ctx, id); err != nil {
		h.reply(msg, userclient.DeleteUserResponse{Error: mapDomainErr(err)})
		return
	}

	h.reply(msg, userclient.DeleteUserResponse{})
	h.publishEvent(userclient.SubjectUserDeleted, userclient.UserDeletedEvent(req))
}

func (h *Handler) reply(msg *nats.Msg, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("failed to marshal RPC response", "error", err)
		return
	}
	if err := msg.Respond(data); err != nil {
		h.logger.Error("failed to send RPC response", "error", err)
	}
}

func (h *Handler) publishEvent(subject string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("failed to marshal event", "subject", subject, "error", err)
		return
	}
	if err := h.nc.Publish(subject, data); err != nil {
		h.logger.Error("failed to publish event", "subject", subject, "error", err)
		return
	}
	h.logger.Info("event published", "subject", subject)
}

func rpcErr(code userclient.ErrorCode, message string) *userclient.RPCError {
	return &userclient.RPCError{Code: code, Message: message}
}

func mapDomainErr(err error) *userclient.RPCError {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		return rpcErr(userclient.ErrCodeNotFound, err.Error())
	case errors.Is(err, domain.ErrEmailAlreadyExists):
		return rpcErr(userclient.ErrCodeAlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidStatus):
		return rpcErr(userclient.ErrCodeValidation, err.Error())
	default:
		return rpcErr(userclient.ErrCodeInternal, "internal server error")
	}
}

func toDTO(u domain.User) userclient.UserDTO {
	return userclient.UserDTO{
		UserID:    u.UserId.String(),
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Email:     u.Email,
		Phone:     u.Phone,
		Age:       u.Age,
		Status:    string(u.Status),
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
		UpdatedAt: u.UpdatedAt.Format(time.RFC3339),
	}
}
