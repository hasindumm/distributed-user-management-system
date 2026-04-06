package ports

import (
	"context"

	"gateway-service/internal/dto"
)

type UserClient interface {
	CreateUser(ctx context.Context, req dto.CreateUserRequest) (dto.UserResponse, error)
	GetUserByID(ctx context.Context, id string) (dto.UserResponse, error)
	GetUserByEmail(ctx context.Context, email string) (dto.UserResponse, error)
	ListUsers(ctx context.Context, req dto.ListUsersRequest) ([]dto.UserResponse, error)
	UpdateUser(ctx context.Context, id string, req dto.UpdateUserRequest) (dto.UserResponse, error)
	DeleteUser(ctx context.Context, id string) error
	Subscribe(handlers EventHandlers) (Subscription, error)
}
