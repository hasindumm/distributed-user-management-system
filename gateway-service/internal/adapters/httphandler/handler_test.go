package httphandler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"gateway-service/internal/adapters/httphandler"
	"gateway-service/internal/dto"
	"gateway-service/internal/ports"
	"user-service/pkg/userclient"
)

type mockUserService struct {
	createUser     func(ctx context.Context, req dto.CreateUserRequest) (dto.UserResponse, error)
	getUserByID    func(ctx context.Context, id string) (dto.UserResponse, error)
	getUserByEmail func(ctx context.Context, email string) (dto.UserResponse, error)
	listUsers      func(ctx context.Context, req dto.ListUsersRequest) ([]dto.UserResponse, error)
	updateUser     func(ctx context.Context, id string, req dto.UpdateUserRequest) (dto.UserResponse, error)
	deleteUser     func(ctx context.Context, id string) error
}

func (m *mockUserService) CreateUser(ctx context.Context, req dto.CreateUserRequest) (dto.UserResponse, error) {
	return m.createUser(ctx, req)
}
func (m *mockUserService) GetUserByID(ctx context.Context, id string) (dto.UserResponse, error) {
	return m.getUserByID(ctx, id)
}
func (m *mockUserService) GetUserByEmail(ctx context.Context, email string) (dto.UserResponse, error) {
	return m.getUserByEmail(ctx, email)
}
func (m *mockUserService) ListUsers(ctx context.Context, req dto.ListUsersRequest) ([]dto.UserResponse, error) {
	return m.listUsers(ctx, req)
}
func (m *mockUserService) UpdateUser(ctx context.Context, id string, req dto.UpdateUserRequest) (dto.UserResponse, error) {
	return m.updateUser(ctx, id, req)
}
func (m *mockUserService) DeleteUser(ctx context.Context, id string) error {
	return m.deleteUser(ctx, id)
}

var _ ports.UserService = (*mockUserService)(nil)

const testUUID = "550e8400-e29b-41d4-a716-446655440000"

var sampleUserResponse = dto.UserResponse{
	UserID:    testUUID,
	FirstName: "John",
	LastName:  "Doe",
	Email:     "john@example.com",
	Status:    "ACTIVE",
	CreatedAt: "2024-01-01T00:00:00Z",
	UpdatedAt: "2024-01-01T00:00:00Z",
}

func newRouter(svc ports.UserService) chi.Router {
	r := chi.NewRouter()
	h := httphandler.NewHttpHandler(svc, noopLogger())
	h.RegisterRoutes(r)
	return r
}

func do(r chi.Router, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeError(t *testing.T, w *httptest.ResponseRecorder) dto.ErrorResponse {
	t.Helper()
	var resp dto.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	return resp
}

func decodeUser(t *testing.T, w *httptest.ResponseRecorder) dto.UserResponse {
	t.Helper()
	var resp dto.UserResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode user response: %v", err)
	}
	return resp
}

func TestHandleHealth(t *testing.T) {
	w := do(newRouter(&mockUserService{}), http.MethodGet, "/api/v1/health", "")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleCreate_Success(t *testing.T) {
	mock := &mockUserService{
		createUser: func(_ context.Context, _ dto.CreateUserRequest) (dto.UserResponse, error) {
			return sampleUserResponse, nil
		},
	}
	body := `{"first_name":"John","last_name":"Doe","email":"john@example.com"}`
	w := do(newRouter(mock), http.MethodPost, "/api/v1/users", body)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	u := decodeUser(t, w)
	if u.Email != "john@example.com" {
		t.Errorf("unexpected email: %s", u.Email)
	}
}

func TestHandleCreate_InvalidJSON(t *testing.T) {
	w := do(newRouter(&mockUserService{}), http.MethodPost, "/api/v1/users", `{bad json`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleCreate_ValidationError(t *testing.T) {
	body := `{"email":"not-an-email"}`
	w := do(newRouter(&mockUserService{}), http.MethodPost, "/api/v1/users", body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	resp := decodeError(t, w)
	if resp.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("unexpected error code: %s", resp.Error.Code)
	}
}

func TestHandleCreate_AlreadyExists(t *testing.T) {
	mock := &mockUserService{
		createUser: func(_ context.Context, _ dto.CreateUserRequest) (dto.UserResponse, error) {
			return dto.UserResponse{}, fmt.Errorf("%w: email already in use", userclient.ErrAlreadyExists)
		},
	}
	body := `{"first_name":"John","last_name":"Doe","email":"john@example.com"}`
	w := do(newRouter(mock), http.MethodPost, "/api/v1/users", body)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
	resp := decodeError(t, w)
	if resp.Error.Code != "ALREADY_EXISTS" {
		t.Errorf("unexpected error code: %s", resp.Error.Code)
	}
}

func TestHandleCreate_InternalError(t *testing.T) {
	mock := &mockUserService{
		createUser: func(_ context.Context, _ dto.CreateUserRequest) (dto.UserResponse, error) {
			return dto.UserResponse{}, fmt.Errorf("%w: something went wrong", userclient.ErrInternal)
		},
	}
	body := `{"first_name":"John","last_name":"Doe","email":"john@example.com"}`
	w := do(newRouter(mock), http.MethodPost, "/api/v1/users", body)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleList_Success(t *testing.T) {
	mock := &mockUserService{
		listUsers: func(_ context.Context, _ dto.ListUsersRequest) ([]dto.UserResponse, error) {
			return []dto.UserResponse{sampleUserResponse}, nil
		},
	}
	w := do(newRouter(mock), http.MethodGet, "/api/v1/users", "")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var users []dto.UserResponse
	if err := json.NewDecoder(w.Body).Decode(&users); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}
}

func TestHandleList_WithStatusFilter(t *testing.T) {
	var capturedReq dto.ListUsersRequest
	mock := &mockUserService{
		listUsers: func(_ context.Context, req dto.ListUsersRequest) ([]dto.UserResponse, error) {
			capturedReq = req
			return []dto.UserResponse{}, nil
		},
	}
	w := do(newRouter(mock), http.MethodGet, "/api/v1/users?status=ACTIVE&limit=10&offset=5", "")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if capturedReq.Status == nil || *capturedReq.Status != "ACTIVE" {
		t.Errorf("expected status filter ACTIVE, got %v", capturedReq.Status)
	}
	if capturedReq.Limit != 10 {
		t.Errorf("expected limit 10, got %d", capturedReq.Limit)
	}
	if capturedReq.Offset != 5 {
		t.Errorf("expected offset 5, got %d", capturedReq.Offset)
	}
}

func TestHandleList_InvalidLimit(t *testing.T) {
	w := do(newRouter(&mockUserService{}), http.MethodGet, "/api/v1/users?limit=abc", "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleList_NegativeOffset(t *testing.T) {
	w := do(newRouter(&mockUserService{}), http.MethodGet, "/api/v1/users?offset=-1", "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleList_ClientError(t *testing.T) {
	mock := &mockUserService{
		listUsers: func(_ context.Context, _ dto.ListUsersRequest) ([]dto.UserResponse, error) {
			return nil, fmt.Errorf("%w: something went wrong", userclient.ErrInternal)
		},
	}
	w := do(newRouter(mock), http.MethodGet, "/api/v1/users", "")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleGetByID_Success(t *testing.T) {
	mock := &mockUserService{
		getUserByID: func(_ context.Context, _ string) (dto.UserResponse, error) {
			return sampleUserResponse, nil
		},
	}
	w := do(newRouter(mock), http.MethodGet, "/api/v1/users/"+testUUID, "")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	u := decodeUser(t, w)
	if u.UserID != testUUID {
		t.Errorf("unexpected user_id: %s", u.UserID)
	}
}

func TestHandleGetByID_InvalidUUID(t *testing.T) {
	w := do(newRouter(&mockUserService{}), http.MethodGet, "/api/v1/users/not-a-uuid", "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	resp := decodeError(t, w)
	if resp.Error.Code != "BAD_REQUEST" {
		t.Errorf("unexpected error code: %s", resp.Error.Code)
	}
}

func TestHandleGetByID_NotFound(t *testing.T) {
	mock := &mockUserService{
		getUserByID: func(_ context.Context, _ string) (dto.UserResponse, error) {
			return dto.UserResponse{}, fmt.Errorf("%w: user not found", userclient.ErrNotFound)
		},
	}
	w := do(newRouter(mock), http.MethodGet, "/api/v1/users/"+testUUID, "")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	resp := decodeError(t, w)
	if resp.Error.Code != "NOT_FOUND" {
		t.Errorf("unexpected error code: %s", resp.Error.Code)
	}
}

func TestHandleGetByEmail_Success(t *testing.T) {
	mock := &mockUserService{
		getUserByEmail: func(_ context.Context, _ string) (dto.UserResponse, error) {
			return sampleUserResponse, nil
		},
	}
	w := do(newRouter(mock), http.MethodGet, "/api/v1/users/email/john@example.com", "")
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleGetByEmail_InvalidEmail(t *testing.T) {
	w := do(newRouter(&mockUserService{}), http.MethodGet, "/api/v1/users/email/not-an-email", "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleGetByEmail_NotFound(t *testing.T) {
	mock := &mockUserService{
		getUserByEmail: func(_ context.Context, _ string) (dto.UserResponse, error) {
			return dto.UserResponse{}, fmt.Errorf("%w: not found", userclient.ErrNotFound)
		},
	}
	w := do(newRouter(mock), http.MethodGet, "/api/v1/users/email/john@example.com", "")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleUpdate_Success(t *testing.T) {
	mock := &mockUserService{
		updateUser: func(_ context.Context, _ string, _ dto.UpdateUserRequest) (dto.UserResponse, error) {
			return sampleUserResponse, nil
		},
	}
	body := `{"first_name":"John","last_name":"Doe","email":"john@example.com","status":"ACTIVE"}`
	w := do(newRouter(mock), http.MethodPut, "/api/v1/users/"+testUUID, body)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleUpdate_InvalidUUID(t *testing.T) {
	body := `{"first_name":"John","last_name":"Doe","email":"john@example.com","status":"ACTIVE"}`
	w := do(newRouter(&mockUserService{}), http.MethodPut, "/api/v1/users/not-a-uuid", body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleUpdate_InvalidJSON(t *testing.T) {
	w := do(newRouter(&mockUserService{}), http.MethodPut, "/api/v1/users/"+testUUID, `{bad`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleUpdate_ValidationError(t *testing.T) {
	body := `{"first_name":"John","last_name":"Doe","email":"john@example.com"}`
	w := do(newRouter(&mockUserService{}), http.MethodPut, "/api/v1/users/"+testUUID, body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleUpdate_NotFound(t *testing.T) {
	mock := &mockUserService{
		updateUser: func(_ context.Context, _ string, _ dto.UpdateUserRequest) (dto.UserResponse, error) {
			return dto.UserResponse{}, fmt.Errorf("%w: not found", userclient.ErrNotFound)
		},
	}
	body := `{"first_name":"John","last_name":"Doe","email":"john@example.com","status":"ACTIVE"}`
	w := do(newRouter(mock), http.MethodPut, "/api/v1/users/"+testUUID, body)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleDelete_Success(t *testing.T) {
	mock := &mockUserService{
		deleteUser: func(_ context.Context, _ string) error { return nil },
	}
	w := do(newRouter(mock), http.MethodDelete, "/api/v1/users/"+testUUID, "")
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestHandleDelete_InvalidUUID(t *testing.T) {
	w := do(newRouter(&mockUserService{}), http.MethodDelete, "/api/v1/users/not-a-uuid", "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleDelete_NotFound(t *testing.T) {
	mock := &mockUserService{
		deleteUser: func(_ context.Context, _ string) error {
			return fmt.Errorf("%w: not found", userclient.ErrNotFound)
		},
	}
	w := do(newRouter(mock), http.MethodDelete, "/api/v1/users/"+testUUID, "")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
