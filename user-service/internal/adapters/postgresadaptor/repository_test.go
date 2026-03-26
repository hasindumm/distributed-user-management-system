package postgresadaptor_test

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"user-service/internal/adapters/postgresadaptor"
	"user-service/internal/domain"
	"user-service/internal/ports"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"testing"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:15",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	if err != nil {
		log.Fatal("failed to start postgres container:", err)
	}

	dbURL, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatal("failed to get connection string:", err)
	}

	testDB, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("failed to open DB:", err)
	}

	if err := testDB.Ping(); err != nil {
		log.Fatal("failed to ping DB:", err)
	}

	_, filename, _, _ := runtime.Caller(0)
	migrationsPath := filepath.Join(filepath.Dir(filename), "../../../db/migrations")
	mig, err := migrate.New("file://"+migrationsPath, dbURL)

	if err != nil {
		log.Fatal("failed to initialize migrations:", err)
	}
	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatal("failed to run migrations:", err)
	}

	code := m.Run()

	if err := pgContainer.Terminate(ctx); err != nil {
		log.Println("failed to terminate container:", err)
	}
	if err := testDB.Close(); err != nil {
		log.Println("failed to close DB:", err)
	}

	os.Exit(code)

}

func cleanDB(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec("TRUNCATE TABLE users")
	if err != nil {
		t.Fatal("failed to clean DB:", err)
	}
}

func createTestUser(t *testing.T, repo ports.UserRepository, email string) domain.User {
	t.Helper()
	phone := "0771234567"
	age := 25
	user := domain.User{
		FirstName: "Test",
		LastName:  "User",
		Email:     email,
		Phone:     &phone,
		Age:       &age,
		Status:    domain.UserStatusActive,
	}
	created, err := repo.Create(context.Background(), user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return created
}

func TestCreate_Success(t *testing.T) {
	cleanDB(t)
	repo := postgresadaptor.NewPostgresRepository(testDB)
	ctx := context.Background()

	phone := "0771234567"
	age := 25
	user := domain.User{
		FirstName: "Hasindu",
		LastName:  "Test",
		Email:     "hasindu@test.com",
		Phone:     &phone,
		Age:       &age,
		Status:    domain.UserStatusActive,
	}

	created, err := repo.Create(ctx, user)

	assert.NoError(t, err)
	assert.NotEmpty(t, created.UserId)
	assert.Equal(t, user.FirstName, created.FirstName)
	assert.Equal(t, user.LastName, created.LastName)
	assert.Equal(t, user.Email, created.Email)
	assert.Equal(t, user.Status, created.Status)
	assert.NotZero(t, created.CreatedAt)
}

func TestCreate_DuplicateEmail(t *testing.T) {
	cleanDB(t)
	repo := postgresadaptor.NewPostgresRepository(testDB)
	ctx := context.Background()

	user := domain.User{
		FirstName: "Test",
		LastName:  "User",
		Email:     "duplicate@test.com",
		Status:    domain.UserStatusActive,
	}

	_, err := repo.Create(ctx, user)
	assert.NoError(t, err)

	_, err = repo.Create(ctx, user)
	assert.True(t, errors.Is(err, domain.ErrEmailAlreadyExists))
}

func TestGetByID_Success(t *testing.T) {
	cleanDB(t)
	repo := postgresadaptor.NewPostgresRepository(testDB)
	ctx := context.Background()

	created := createTestUser(t, repo, "getbyid@test.com")

	found, err := repo.GetByID(ctx, created.UserId)

	assert.NoError(t, err)
	assert.Equal(t, created.UserId, found.UserId)
	assert.Equal(t, created.Email, found.Email)
}

func TestGetByID_NotFound(t *testing.T) {
	cleanDB(t)
	repo := postgresadaptor.NewPostgresRepository(testDB)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())

	assert.True(t, errors.Is(err, domain.ErrUserNotFound))
}

func TestGetByEmail_Success(t *testing.T) {
	cleanDB(t)
	repo := postgresadaptor.NewPostgresRepository(testDB)
	ctx := context.Background()

	created := createTestUser(t, repo, "getbyemail@test.com")

	found, err := repo.GetByEmail(ctx, created.Email)

	assert.NoError(t, err)
	assert.Equal(t, created.UserId, found.UserId)
}

func TestGetByEmail_NotFound(t *testing.T) {
	cleanDB(t)
	repo := postgresadaptor.NewPostgresRepository(testDB)
	ctx := context.Background()

	_, err := repo.GetByEmail(ctx, "notexist@test.com")

	assert.True(t, errors.Is(err, domain.ErrUserNotFound))
}

func TestList_Success(t *testing.T) {
	cleanDB(t)
	repo := postgresadaptor.NewPostgresRepository(testDB)
	ctx := context.Background()

	createTestUser(t, repo, "user1@test.com")
	createTestUser(t, repo, "user2@test.com")
	createTestUser(t, repo, "user3@test.com")

	users, err := repo.List(ctx, nil, 10, 0)

	assert.NoError(t, err)
	assert.Len(t, users, 3)
}

func TestList_FilterByStatus(t *testing.T) {
	cleanDB(t)
	repo := postgresadaptor.NewPostgresRepository(testDB)
	ctx := context.Background()

	createTestUser(t, repo, "active@test.com")

	inactiveUser := domain.User{
		FirstName: "Inactive",
		LastName:  "User",
		Email:     "inactive@test.com",
		Status:    domain.UserStatusInactive,
	}
	_, err := repo.Create(ctx, inactiveUser)
	assert.NoError(t, err)

	status := domain.UserStatusInactive
	users, err := repo.List(ctx, &status, 10, 0)

	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, domain.UserStatusInactive, users[0].Status)
}

func TestList_Pagination(t *testing.T) {
	cleanDB(t)
	repo := postgresadaptor.NewPostgresRepository(testDB)
	ctx := context.Background()

	createTestUser(t, repo, "user1@test.com")
	createTestUser(t, repo, "user2@test.com")
	createTestUser(t, repo, "user3@test.com")

	page1, err := repo.List(ctx, nil, 2, 0)
	assert.NoError(t, err)
	assert.Len(t, page1, 2)

	page2, err := repo.List(ctx, nil, 2, 2)
	assert.NoError(t, err)
	assert.Len(t, page2, 1)
}

func TestUpdate_Success(t *testing.T) {
	cleanDB(t)
	repo := postgresadaptor.NewPostgresRepository(testDB)
	ctx := context.Background()

	created := createTestUser(t, repo, "update@test.com")
	created.FirstName = "Updated"
	created.Status = domain.UserStatusInactive

	updated, err := repo.Update(ctx, created)

	assert.NoError(t, err)
	assert.Equal(t, "Updated", updated.FirstName)
	assert.Equal(t, domain.UserStatusInactive, updated.Status)
}

func TestUpdate_NotFound(t *testing.T) {
	cleanDB(t)
	repo := postgresadaptor.NewPostgresRepository(testDB)
	ctx := context.Background()

	user := domain.User{
		UserId:    uuid.New(),
		FirstName: "Ghost",
		LastName:  "User",
		Email:     "ghost@test.com",
		Status:    domain.UserStatusActive,
	}

	_, err := repo.Update(ctx, user)

	assert.True(t, errors.Is(err, domain.ErrUserNotFound))
}

func TestDelete_Success(t *testing.T) {
	cleanDB(t)
	repo := postgresadaptor.NewPostgresRepository(testDB)
	ctx := context.Background()

	created := createTestUser(t, repo, "delete@test.com")

	err := repo.Delete(ctx, created.UserId)
	assert.NoError(t, err)

	_, err = repo.GetByID(ctx, created.UserId)
	assert.True(t, errors.Is(err, domain.ErrUserNotFound))
}

func TestDelete_NotFound(t *testing.T) {
	cleanDB(t)
	repo := postgresadaptor.NewPostgresRepository(testDB)
	ctx := context.Background()

	err := repo.Delete(ctx, uuid.New())

	assert.True(t, errors.Is(err, domain.ErrUserNotFound))
}
