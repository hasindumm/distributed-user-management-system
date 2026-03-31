package ports

import (
	"context"
	"user-service/internal/domain"

	"github.com/google/uuid"
)

type UserService interface {
	CreateUser(ctx context.Context, firstName, lastName, email string, phone *string, age *int, status domain.UserStatus) (domain.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	ListUsers(ctx context.Context, status *domain.UserStatus, limit, offset int32) ([]domain.User, error)
	UpdateUser(ctx context.Context, user domain.User) (domain.User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
}
