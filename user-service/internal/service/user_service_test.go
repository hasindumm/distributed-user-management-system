package service_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"
	"user-service/internal/domain"
	"user-service/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRepo struct {
	createFn     func(ctx context.Context, user domain.User) (domain.User, error)
	getByIDFn    func(ctx context.Context, id uuid.UUID) (domain.User, error)
	getByEmailFn func(ctx context.Context, email string) (domain.User, error)
	listFn       func(ctx context.Context, status *domain.UserStatus, limit, offset int32) ([]domain.User, error)
	updateFn     func(ctx context.Context, user domain.User) (domain.User, error)
	deleteFn     func(ctx context.Context, id uuid.UUID) error
}

func (m *mockRepo) Create(ctx context.Context, user domain.User) (domain.User, error) {
	return m.createFn(ctx, user)
}

func (m *mockRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	return m.getByIDFn(ctx, id)
}

func (m *mockRepo) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	return m.getByEmailFn(ctx, email)
}

func (m *mockRepo) List(ctx context.Context, status *domain.UserStatus, limit, offset int32) ([]domain.User, error) {
	return m.listFn(ctx, status, limit, offset)
}

func (m *mockRepo) Update(ctx context.Context, user domain.User) (domain.User, error) {
	return m.updateFn(ctx, user)
}

func (m *mockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.deleteFn(ctx, id)
}

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func sampleUser() domain.User {
	phone := "+94771234567"
	age := 30
	return domain.User{
		UserId:    uuid.New(),
		FirstName: "hasindu",
		LastName:  "muhandiram",
		Email:     "hasindu@example.com",
		Phone:     &phone,
		Age:       &age,
		Status:    domain.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestCreateUser_Success(t *testing.T) {
	want := sampleUser()
	repo := &mockRepo{
		createFn: func(_ context.Context, _ domain.User) (domain.User, error) {
			return want, nil
		},
	}
	svc := service.New(repo, testLogger)

	got, err := svc.CreateUser(context.Background(), want.FirstName, want.LastName, want.Email, want.Phone, want.Age, want.Status)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	repo := &mockRepo{
		createFn: func(_ context.Context, _ domain.User) (domain.User, error) {
			return domain.User{}, domain.ErrEmailAlreadyExists
		},
	}
	svc := service.New(repo, testLogger)

	_, err := svc.CreateUser(context.Background(), "A", "B", "dup@example.com", nil, nil, domain.UserStatusActive)
	assert.ErrorIs(t, err, domain.ErrEmailAlreadyExists)
}

func TestGetUserByID_Success(t *testing.T) {
	want := sampleUser()
	repo := &mockRepo{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (domain.User, error) {
			return want, nil
		},
	}
	svc := service.New(repo, testLogger)

	got, err := svc.GetUserByID(context.Background(), want.UserId)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGetUserByID_NotFound(t *testing.T) {
	repo := &mockRepo{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (domain.User, error) {
			return domain.User{}, domain.ErrUserNotFound
		},
	}
	svc := service.New(repo, testLogger)

	_, err := svc.GetUserByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestGetUserByEmail_Success(t *testing.T) {
	want := sampleUser()
	repo := &mockRepo{
		getByEmailFn: func(_ context.Context, _ string) (domain.User, error) {
			return want, nil
		},
	}
	svc := service.New(repo, testLogger)

	got, err := svc.GetUserByEmail(context.Background(), want.Email)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGetUserByEmail_NotFound(t *testing.T) {
	repo := &mockRepo{
		getByEmailFn: func(_ context.Context, _ string) (domain.User, error) {
			return domain.User{}, domain.ErrUserNotFound
		},
	}
	svc := service.New(repo, testLogger)

	_, err := svc.GetUserByEmail(context.Background(), "missing@example.com")
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestListUsers_Success(t *testing.T) {
	users := []domain.User{sampleUser(), sampleUser()}
	repo := &mockRepo{
		listFn: func(_ context.Context, _ *domain.UserStatus, _, _ int32) ([]domain.User, error) {
			return users, nil
		},
	}
	svc := service.New(repo, testLogger)

	got, err := svc.ListUsers(context.Background(), nil, 10, 0)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestListUsers_RepoError(t *testing.T) {
	repoErr := errors.New("db error")
	repo := &mockRepo{
		listFn: func(_ context.Context, _ *domain.UserStatus, _, _ int32) ([]domain.User, error) {
			return nil, repoErr
		},
	}
	svc := service.New(repo, testLogger)

	_, err := svc.ListUsers(context.Background(), nil, 10, 0)
	assert.ErrorIs(t, err, repoErr)
}

func TestUpdateUser_Success(t *testing.T) {
	want := sampleUser()
	repo := &mockRepo{
		updateFn: func(_ context.Context, u domain.User) (domain.User, error) {
			return u, nil
		},
	}
	svc := service.New(repo, testLogger)

	got, err := svc.UpdateUser(context.Background(), want)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestUpdateUser_NotFound(t *testing.T) {
	repo := &mockRepo{
		updateFn: func(_ context.Context, _ domain.User) (domain.User, error) {
			return domain.User{}, domain.ErrUserNotFound
		},
	}
	svc := service.New(repo, testLogger)

	_, err := svc.UpdateUser(context.Background(), sampleUser())
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestDeleteUser_Success(t *testing.T) {
	repo := &mockRepo{
		deleteFn: func(_ context.Context, _ uuid.UUID) error {
			return nil
		},
	}
	svc := service.New(repo, testLogger)

	err := svc.DeleteUser(context.Background(), uuid.New())
	require.NoError(t, err)
}

func TestDeleteUser_NotFound(t *testing.T) {
	repo := &mockRepo{
		deleteFn: func(_ context.Context, _ uuid.UUID) error {
			return domain.ErrUserNotFound
		},
	}
	svc := service.New(repo, testLogger)

	err := svc.DeleteUser(context.Background(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}
