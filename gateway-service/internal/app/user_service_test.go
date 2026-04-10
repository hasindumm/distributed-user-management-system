package app_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"gateway-service/internal/app"
	"gateway-service/internal/dto"
	"gateway-service/internal/ports"
	"user-service/pkg/userclient"
)

type mockUserClient struct {
	createUser     func(ctx context.Context, req dto.CreateUserRequest) (dto.UserResponse, error)
	getUserByID    func(ctx context.Context, id string) (dto.UserResponse, error)
	getUserByEmail func(ctx context.Context, email string) (dto.UserResponse, error)
	listUsers      func(ctx context.Context, req dto.ListUsersRequest) ([]dto.UserResponse, error)
	updateUser     func(ctx context.Context, id string, req dto.UpdateUserRequest) (dto.UserResponse, error)
	deleteUser     func(ctx context.Context, id string) error
	subscribe      func(ch chan<- userclient.Event) (ports.Subscription, error)
}

func (m *mockUserClient) CreateUser(ctx context.Context, req dto.CreateUserRequest) (dto.UserResponse, error) {
	return m.createUser(ctx, req)
}
func (m *mockUserClient) GetUserByID(ctx context.Context, id string) (dto.UserResponse, error) {
	return m.getUserByID(ctx, id)
}
func (m *mockUserClient) GetUserByEmail(ctx context.Context, email string) (dto.UserResponse, error) {
	return m.getUserByEmail(ctx, email)
}
func (m *mockUserClient) ListUsers(ctx context.Context, req dto.ListUsersRequest) ([]dto.UserResponse, error) {
	return m.listUsers(ctx, req)
}
func (m *mockUserClient) UpdateUser(ctx context.Context, id string, req dto.UpdateUserRequest) (dto.UserResponse, error) {
	return m.updateUser(ctx, id, req)
}
func (m *mockUserClient) DeleteUser(ctx context.Context, id string) error {
	return m.deleteUser(ctx, id)
}
func (m *mockUserClient) Subscribe(ch chan<- userclient.Event) (ports.Subscription, error) {
	return m.subscribe(ch)
}

var _ ports.UserClient = (*mockUserClient)(nil)

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

var sampleResp = dto.UserResponse{
	UserID:    "a7f5b6e2-1234-4567-89ab-cdef01234567",
	FirstName: "Test",
	LastName:  "User",
	Email:     "test@example.com",
	Status:    "ACTIVE",
	CreatedAt: "2024-01-01T00:00:00Z",
	UpdatedAt: "2024-01-01T00:00:00Z",
}

func TestApp_CreateUser_Success(t *testing.T) {
	client := &mockUserClient{
		createUser: func(_ context.Context, _ dto.CreateUserRequest) (dto.UserResponse, error) {
			return sampleResp, nil
		},
	}
	svc := app.NewApp(client, noopLogger())

	got, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{
		FirstName: "Test", LastName: "User", Email: "test@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Email != sampleResp.Email {
		t.Errorf("expected email %s, got %s", sampleResp.Email, got.Email)
	}
}

func TestApp_CreateUser_Error(t *testing.T) {
	wantErr := errors.New("create failed")
	client := &mockUserClient{
		createUser: func(_ context.Context, _ dto.CreateUserRequest) (dto.UserResponse, error) {
			return dto.UserResponse{}, wantErr
		},
	}
	svc := app.NewApp(client, noopLogger())

	_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected error %v, got %v", wantErr, err)
	}
}

func TestApp_GetUserByID_Success(t *testing.T) {
	client := &mockUserClient{
		getUserByID: func(_ context.Context, _ string) (dto.UserResponse, error) {
			return sampleResp, nil
		},
	}
	svc := app.NewApp(client, noopLogger())

	got, err := svc.GetUserByID(context.Background(), sampleResp.UserID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != sampleResp.UserID {
		t.Errorf("expected userID %s, got %s", sampleResp.UserID, got.UserID)
	}
}

func TestApp_GetUserByID_NotFound(t *testing.T) {
	wantErr := errors.New("not found")
	client := &mockUserClient{
		getUserByID: func(_ context.Context, _ string) (dto.UserResponse, error) {
			return dto.UserResponse{}, wantErr
		},
	}
	svc := app.NewApp(client, noopLogger())

	_, err := svc.GetUserByID(context.Background(), "missing-id")
	if !errors.Is(err, wantErr) {
		t.Errorf("expected error %v, got %v", wantErr, err)
	}
}

func TestApp_ListUsers_Success(t *testing.T) {
	client := &mockUserClient{
		listUsers: func(_ context.Context, _ dto.ListUsersRequest) ([]dto.UserResponse, error) {
			return []dto.UserResponse{sampleResp, sampleResp}, nil
		},
	}
	svc := app.NewApp(client, noopLogger())

	got, err := svc.ListUsers(context.Background(), dto.ListUsersRequest{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 users, got %d", len(got))
	}
}

func TestApp_GetUserByEmail_Success(t *testing.T) {
	client := &mockUserClient{
		getUserByEmail: func(_ context.Context, _ string) (dto.UserResponse, error) {
			return sampleResp, nil
		},
	}
	svc := app.NewApp(client, noopLogger())

	got, err := svc.GetUserByEmail(context.Background(), sampleResp.Email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Email != sampleResp.Email {
		t.Errorf("expected email %s, got %s", sampleResp.Email, got.Email)
	}
}

func TestApp_UpdateUser_Success(t *testing.T) {
	client := &mockUserClient{
		updateUser: func(_ context.Context, _ string, _ dto.UpdateUserRequest) (dto.UserResponse, error) {
			return sampleResp, nil
		},
	}
	svc := app.NewApp(client, noopLogger())

	got, err := svc.UpdateUser(context.Background(), sampleResp.UserID, dto.UpdateUserRequest{
		FirstName: "Test", LastName: "User", Email: sampleResp.Email, Status: "ACTIVE",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != sampleResp.UserID {
		t.Errorf("expected userID %s, got %s", sampleResp.UserID, got.UserID)
	}
}

func TestApp_DeleteUser_Success(t *testing.T) {
	client := &mockUserClient{
		deleteUser: func(_ context.Context, _ string) error { return nil },
	}
	svc := app.NewApp(client, noopLogger())

	if err := svc.DeleteUser(context.Background(), sampleResp.UserID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
