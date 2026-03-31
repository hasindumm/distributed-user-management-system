package postgresadaptor

import (
	"context"
	"database/sql"
	"errors"
	"user-service/internal/adapters/postgresadaptor/db"
	"user-service/internal/domain"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type PostgresUserRepository struct {
	queries *db.Queries
}

func NewPostgresRepository(database *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{
		queries: db.New(database),
	}
}

func (r *PostgresUserRepository) Create(ctx context.Context, user domain.User) (domain.User, error) {
	params := db.CreateUserParams{
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Phone:     toNullString(user.Phone),
		Age:       toNullInt32(user.Age),
		Status:    string(user.Status),
	}
	result, err := r.queries.CreateUser(ctx, params)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return domain.User{}, domain.ErrEmailAlreadyExists
		}
		return domain.User{}, err
	}
	return toDomainUser(result), nil

}

func (r *PostgresUserRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {

	result, err := r.queries.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, domain.ErrUserNotFound
		}
	}
	return toDomainUser(result), nil

}

func (r *PostgresUserRepository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	result, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, err
	}
	return toDomainUser(result), nil
}

func (r *PostgresUserRepository) List(ctx context.Context, status *domain.UserStatus, limit, offset int32) ([]domain.User, error) {

	var statusFilter sql.NullString
	if status != nil {
		statusFilter = sql.NullString{
			String: string(*status),
			Valid:  true,
		}
	}

	args := db.ListUsersParams{
		Status: statusFilter,
		Limit:  limit,
		Offset: offset,
	}
	result, err := r.queries.ListUsers(ctx, args)

	if err != nil {
		return []domain.User{}, err
	}

	users := make([]domain.User, len(result))
	for i, result := range result {
		users[i] = toDomainUser(result)
	}
	return users, nil

}

func (r *PostgresUserRepository) Update(ctx context.Context, user domain.User) (domain.User, error) {

	args := db.UpdateUserParams{
		UserID:    user.UserId,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Phone:     toNullString(user.Phone),
		Age:       toNullInt32(user.Age),
		Status:    string(user.Status),
	}
	result, err := r.queries.UpdateUser(ctx, args)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, domain.ErrUserNotFound
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return domain.User{}, domain.ErrEmailAlreadyExists
		}
	}
	return toDomainUser(result), nil

}
func (r *PostgresUserRepository) Delete(ctx context.Context, id uuid.UUID) error {

	result, err := r.queries.DeleteUser(ctx, id)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}
