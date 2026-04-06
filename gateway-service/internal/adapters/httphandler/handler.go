package httphandler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	"gateway-service/internal/dto"
	"gateway-service/internal/ports"
)

type Handler struct {
	service  ports.UserService
	validate *validator.Validate
	logger   *slog.Logger
}

func New(service ports.UserService, logger *slog.Logger) *Handler {
	return &Handler{
		service:  service,
		validate: validator.New(),
		logger:   logger,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/health", h.handleHealth)

	r.Post("/api/v1/users", h.handleCreate)
	r.Get("/api/v1/users", h.handleList)
	r.Get("/api/v1/users/email/{email}", h.handleGetByEmail)
	r.Get("/api/v1/users/{id}", h.handleGetByID)
	r.Put("/api/v1/users/{id}", h.handleUpdate)
	r.Delete("/api/v1/users/{id}", h.handleDelete)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WarnContext(r.Context(), "create: invalid JSON body", "error", err)
		writeJSON(w, http.StatusBadRequest, errorResponse("BAD_REQUEST", "invalid JSON body"))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		h.logger.WarnContext(r.Context(), "create: validation failed", "error", err)
		writeJSON(w, http.StatusBadRequest, errorResponse("VALIDATION_ERROR", err.Error()))
		return
	}

	user, err := h.service.CreateUser(r.Context(), req)
	if err != nil {
		status, body := mapClientError(err)
		h.logger.WarnContext(r.Context(), "create: service error", "error", err)
		writeJSON(w, status, body)
		return
	}

	h.logger.InfoContext(r.Context(), "create: user created", "user_id", user.UserID)
	writeJSON(w, http.StatusCreated, user)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	req, ok := parseListRequest(w, r, h)
	if !ok {
		return
	}

	users, err := h.service.ListUsers(r.Context(), req)
	if err != nil {
		status, body := mapClientError(err)
		h.logger.WarnContext(r.Context(), "list: service error", "error", err)
		writeJSON(w, status, body)
		return
	}

	h.logger.InfoContext(r.Context(), "list: users returned", "count", len(users))
	writeJSON(w, http.StatusOK, users)
}

func (h *Handler) handleGetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.validate.Var(id, "required,uuid"); err != nil {
		h.logger.WarnContext(r.Context(), "getByID: invalid uuid", "id", id)
		writeJSON(w, http.StatusBadRequest, errorResponse("BAD_REQUEST", "invalid user id: must be a valid UUID"))
		return
	}

	user, err := h.service.GetUserByID(r.Context(), id)
	if err != nil {
		status, body := mapClientError(err)
		h.logger.WarnContext(r.Context(), "getByID: service error", "error", err, "user_id", id)
		writeJSON(w, status, body)
		return
	}

	h.logger.InfoContext(r.Context(), "getByID: user found", "user_id", id)
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) handleGetByEmail(w http.ResponseWriter, r *http.Request) {
	email := chi.URLParam(r, "email")
	if err := h.validate.Var(email, "required,email"); err != nil {
		h.logger.WarnContext(r.Context(), "getByEmail: invalid email", "email", email)
		writeJSON(w, http.StatusBadRequest, errorResponse("BAD_REQUEST", "invalid email format"))
		return
	}

	user, err := h.service.GetUserByEmail(r.Context(), email)
	if err != nil {
		status, body := mapClientError(err)
		h.logger.WarnContext(r.Context(), "getByEmail: service error", "error", err, "email", email)
		writeJSON(w, status, body)
		return
	}

	h.logger.InfoContext(r.Context(), "getByEmail: user found", "email", email)
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.validate.Var(id, "required,uuid"); err != nil {
		h.logger.WarnContext(r.Context(), "update: invalid uuid", "id", id)
		writeJSON(w, http.StatusBadRequest, errorResponse("BAD_REQUEST", "invalid user id: must be a valid UUID"))
		return
	}

	var req dto.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WarnContext(r.Context(), "update: invalid JSON body", "error", err)
		writeJSON(w, http.StatusBadRequest, errorResponse("BAD_REQUEST", "invalid JSON body"))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		h.logger.WarnContext(r.Context(), "update: validation failed", "error", err)
		writeJSON(w, http.StatusBadRequest, errorResponse("VALIDATION_ERROR", err.Error()))
		return
	}

	user, err := h.service.UpdateUser(r.Context(), id, req)
	if err != nil {
		status, body := mapClientError(err)
		h.logger.WarnContext(r.Context(), "update: service error", "error", err, "user_id", id)
		writeJSON(w, status, body)
		return
	}

	h.logger.InfoContext(r.Context(), "update: user updated", "user_id", id)
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.validate.Var(id, "required,uuid"); err != nil {
		h.logger.WarnContext(r.Context(), "delete: invalid uuid", "id", id)
		writeJSON(w, http.StatusBadRequest, errorResponse("BAD_REQUEST", "invalid user id: must be a valid UUID"))
		return
	}

	if err := h.service.DeleteUser(r.Context(), id); err != nil {
		status, body := mapClientError(err)
		h.logger.WarnContext(r.Context(), "delete: service error", "error", err, "user_id", id)
		writeJSON(w, status, body)
		return
	}

	h.logger.InfoContext(r.Context(), "delete: user deleted", "user_id", id)
	w.WriteHeader(http.StatusNoContent)
}

func parseListRequest(w http.ResponseWriter, r *http.Request, h *Handler) (dto.ListUsersRequest, bool) {
	const defaultLimit = 20

	req := dto.ListUsersRequest{
		Limit:  defaultLimit,
		Offset: 0,
	}

	if s := r.URL.Query().Get("status"); s != "" {
		req.Status = &s
	}

	if l := r.URL.Query().Get("limit"); l != "" {
		v, err := strconv.ParseInt(l, 10, 32)
		if err != nil || v < 1 {
			writeJSON(w, http.StatusBadRequest, errorResponse("BAD_REQUEST", "limit must be a positive integer"))
			return dto.ListUsersRequest{}, false
		}
		req.Limit = int32(v)
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		v, err := strconv.ParseInt(o, 10, 32)
		if err != nil || v < 0 {
			writeJSON(w, http.StatusBadRequest, errorResponse("BAD_REQUEST", "offset must be a non-negative integer"))
			return dto.ListUsersRequest{}, false
		}
		req.Offset = int32(v)
	}

	if err := h.validate.Struct(req); err != nil {
		h.logger.WarnContext(r.Context(), "list: validation failed", "error", err)
		writeJSON(w, http.StatusBadRequest, errorResponse("VALIDATION_ERROR", err.Error()))
		return dto.ListUsersRequest{}, false
	}

	return req, true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("writeJSON: failed to encode response", "error", err)
	}
}
