package service

import (
	"context"
	"log/slog"
	"user-service/internal/domain"
	"user-service/internal/ports"

	"github.com/google/uuid"
)

type UserService struct {
	repo   ports.UserRepository
	logger *slog.Logger
}

func New(repo ports.UserRepository, logger *slog.Logger) *UserService {
	return &UserService{
		repo:   repo,
		logger: logger,
	}
}

func (s *UserService) CreateUser(ctx context.Context, firstName, lastName, email string, phone *string, age *int, status domain.UserStatus) (domain.User, error) {
	s.logger.InfoContext(ctx, "creating user", "email", email)

	user := domain.User{
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,
		Phone:     phone,
		Age:       age,
		Status:    status,
	}

	created, err := s.repo.Create(ctx, user)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create user", "email", email, "error", err)
		return domain.User{}, err
	}

	s.logger.InfoContext(ctx, "user created", "user_id", created.UserId)
	return created, nil
}

func (s *UserService) GetUserByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	s.logger.InfoContext(ctx, "getting user by id", "user_id", id)

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get user by id", "user_id", id, "error", err)
		return domain.User{}, err
	}

	return user, nil
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	s.logger.InfoContext(ctx, "getting user by email", "email", email)

	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get user by email", "email", email, "error", err)
		return domain.User{}, err
	}

	return user, nil
}

func (s *UserService) ListUsers(ctx context.Context, status *domain.UserStatus, limit, offset int32) ([]domain.User, error) {
	s.logger.InfoContext(ctx, "listing users", "limit", limit, "offset", offset)

	users, err := s.repo.List(ctx, status, limit, offset)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list users", "error", err)
		return nil, err
	}

	return users, nil
}

func (s *UserService) UpdateUser(ctx context.Context, user domain.User) (domain.User, error) {
	s.logger.InfoContext(ctx, "updating user", "user_id", user.UserId)

	updated, err := s.repo.Update(ctx, user)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to update user", "user_id", user.UserId, "error", err)
		return domain.User{}, err
	}

	s.logger.InfoContext(ctx, "user updated", "user_id", updated.UserId)
	return updated, nil
}

func (s *UserService) DeleteUser(ctx context.Context, id uuid.UUID) error {
	s.logger.InfoContext(ctx, "deleting user", "user_id", id)

	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete user", "user_id", id, "error", err)
		return err
	}

	s.logger.InfoContext(ctx, "user deleted", "user_id", id)
	return nil
}
