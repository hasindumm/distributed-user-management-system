package app

import (
	"context"
	"log/slog"

	"gateway-service/internal/dto"
	"gateway-service/internal/ports"
)

type UserService struct {
	client ports.UserClient
	logger *slog.Logger
}

func NewApp(client ports.UserClient, logger *slog.Logger) *UserService {
	return &UserService{client: client, logger: logger}
}

func (s *UserService) CreateUser(ctx context.Context, req dto.CreateUserRequest) (dto.UserResponse, error) {
	return s.client.CreateUser(ctx, req)
}

func (s *UserService) GetUserByID(ctx context.Context, id string) (dto.UserResponse, error) {
	return s.client.GetUserByID(ctx, id)
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (dto.UserResponse, error) {
	return s.client.GetUserByEmail(ctx, email)
}

func (s *UserService) ListUsers(ctx context.Context, req dto.ListUsersRequest) ([]dto.UserResponse, error) {
	return s.client.ListUsers(ctx, req)
}

func (s *UserService) UpdateUser(ctx context.Context, id string, req dto.UpdateUserRequest) (dto.UserResponse, error) {
	return s.client.UpdateUser(ctx, id, req)
}

func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	return s.client.DeleteUser(ctx, id)
}
