package natsadaptor_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"user-service/internal/adapters/natsadaptor"
	"user-service/internal/domain"
	"user-service/internal/ports"
	"user-service/pkg/userclient"

	"github.com/google/uuid"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUserService struct {
	createUser     func(ctx context.Context, firstName, lastName, email string, phone *string, age *int, status domain.UserStatus) (domain.User, error)
	getUserByID    func(ctx context.Context, id uuid.UUID) (domain.User, error)
	getUserByEmail func(ctx context.Context, email string) (domain.User, error)
	listUsers      func(ctx context.Context, status *domain.UserStatus, limit, offset int32) ([]domain.User, error)
	updateUser     func(ctx context.Context, user domain.User) (domain.User, error)
	deleteUser     func(ctx context.Context, id uuid.UUID) error
}

func (m *mockUserService) CreateUser(ctx context.Context, firstName, lastName, email string, phone *string, age *int, status domain.UserStatus) (domain.User, error) {
	return m.createUser(ctx, firstName, lastName, email, phone, age, status)
}
func (m *mockUserService) GetUserByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	return m.getUserByID(ctx, id)
}
func (m *mockUserService) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	return m.getUserByEmail(ctx, email)
}
func (m *mockUserService) ListUsers(ctx context.Context, status *domain.UserStatus, limit, offset int32) ([]domain.User, error) {
	return m.listUsers(ctx, status, limit, offset)
}
func (m *mockUserService) ListAllUsers(ctx context.Context) ([]domain.User, error) {
	return nil, nil
}
func (m *mockUserService) UpdateUser(ctx context.Context, user domain.User) (domain.User, error) {
	return m.updateUser(ctx, user)
}
func (m *mockUserService) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return m.deleteUser(ctx, id)
}

var _ ports.UserService = (*mockUserService)(nil)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()                     //nolint:errcheck
	return l.Addr().(*net.TCPAddr).Port //nolint:errcheck
}

func startNATSServer(t *testing.T) (string, func()) {
	t.Helper()
	port := freePort(t)
	opts := &natsserver.Options{Port: port, NoLog: true, NoSigs: true}
	srv, err := natsserver.NewServer(opts)
	require.NoError(t, err)
	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server did not become ready")
	}
	return fmt.Sprintf("nats://127.0.0.1:%d", port), func() { srv.Shutdown() }
}

func newTestHandler(t *testing.T, nc *nats.Conn, svc ports.UserService) *natsadaptor.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := natsadaptor.NewHandler(svc, nc, logger)
	require.NoError(t, h.Subscribe())
	return h
}

func doRequest(t *testing.T, nc *nats.Conn, subject string, payload, out any) {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	msg, err := nc.RequestWithContext(context.Background(), subject, data)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(msg.Data, out))
}

func sampleDomainUser() domain.User {
	return domain.User{
		UserId:    uuid.MustParse("a7f5b6e2-1234-4567-89ab-cdef01234567"),
		FirstName: "Test",
		LastName:  "User",
		Email:     "test@example.com",
		Status:    domain.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

const testUUID = "a7f5b6e2-1234-4567-89ab-cdef01234567"

func TestHandler_HandleCreate_Success(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	want := sampleDomainUser()
	svc := &mockUserService{
		createUser: func(_ context.Context, _, _, _ string, _ *string, _ *int, _ domain.UserStatus) (domain.User, error) {
			return want, nil
		},
	}
	newTestHandler(t, nc, svc)

	var resp userclient.CreateUserResponse
	doRequest(t, nc, userclient.SubjectCreateUser, userclient.CreateUserRequest{
		FirstName: "Test", LastName: "User", Email: "test@example.com",
	}, &resp)

	require.Nil(t, resp.Error)
	require.NotNil(t, resp.User)
	assert.Equal(t, want.Email, resp.User.Email)
	assert.Equal(t, want.UserId.String(), resp.User.UserID)
}

func TestHandler_HandleCreate_InvalidJSON(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	newTestHandler(t, nc, &mockUserService{})

	msg, err := nc.RequestWithContext(context.Background(), userclient.SubjectCreateUser, []byte(`{bad json`))
	require.NoError(t, err)

	var resp userclient.CreateUserResponse
	require.NoError(t, json.Unmarshal(msg.Data, &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, userclient.ErrCodeValidation, resp.Error.Code)
}

func TestHandler_HandleCreate_ValidationError(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	newTestHandler(t, nc, &mockUserService{})

	var resp userclient.CreateUserResponse
	doRequest(t, nc, userclient.SubjectCreateUser, userclient.CreateUserRequest{Email: "not-an-email"}, &resp)
	require.NotNil(t, resp.Error)
	assert.Equal(t, userclient.ErrCodeValidation, resp.Error.Code)
}

func TestHandler_HandleCreate_ServiceError_EmailExists(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	svc := &mockUserService{
		createUser: func(_ context.Context, _, _, _ string, _ *string, _ *int, _ domain.UserStatus) (domain.User, error) {
			return domain.User{}, domain.ErrEmailAlreadyExists
		},
	}
	newTestHandler(t, nc, svc)

	var resp userclient.CreateUserResponse
	doRequest(t, nc, userclient.SubjectCreateUser, userclient.CreateUserRequest{
		FirstName: "Test", LastName: "User", Email: "dup@example.com",
	}, &resp)

	require.NotNil(t, resp.Error)
	assert.Equal(t, userclient.ErrCodeAlreadyExists, resp.Error.Code)
}

func TestHandler_HandleCreate_PublishesCreatedEvent(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	eventCh := make(chan userclient.UserCreatedEvent, 1)
	_, err = nc.Subscribe(userclient.SubjectUserCreated, func(msg *nats.Msg) {
		var evt userclient.UserCreatedEvent
		if json.Unmarshal(msg.Data, &evt) == nil {
			eventCh <- evt
		}
	})
	require.NoError(t, err)
	require.NoError(t, nc.Flush())

	want := sampleDomainUser()
	svc := &mockUserService{
		createUser: func(_ context.Context, _, _, _ string, _ *string, _ *int, _ domain.UserStatus) (domain.User, error) {
			return want, nil
		},
	}
	newTestHandler(t, nc, svc)

	var resp userclient.CreateUserResponse
	doRequest(t, nc, userclient.SubjectCreateUser, userclient.CreateUserRequest{
		FirstName: "Test", LastName: "User", Email: "test@example.com",
	}, &resp)
	require.Nil(t, resp.Error)

	select {
	case evt := <-eventCh:
		assert.Equal(t, want.Email, evt.User.Email)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for created event")
	}
}

// --- get by ID ---

func TestHandler_HandleGetByID_Success(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	want := sampleDomainUser()
	svc := &mockUserService{
		getUserByID: func(_ context.Context, _ uuid.UUID) (domain.User, error) {
			return want, nil
		},
	}
	newTestHandler(t, nc, svc)

	var resp userclient.GetUserByIDResponse
	doRequest(t, nc, userclient.SubjectGetUserByID, userclient.GetUserByIDRequest{UserID: testUUID}, &resp)

	require.Nil(t, resp.Error)
	require.NotNil(t, resp.User)
	assert.Equal(t, want.Email, resp.User.Email)
}

func TestHandler_HandleGetByID_NotFound(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	svc := &mockUserService{
		getUserByID: func(_ context.Context, _ uuid.UUID) (domain.User, error) {
			return domain.User{}, domain.ErrUserNotFound
		},
	}
	newTestHandler(t, nc, svc)

	var resp userclient.GetUserByIDResponse
	doRequest(t, nc, userclient.SubjectGetUserByID, userclient.GetUserByIDRequest{UserID: testUUID}, &resp)

	require.NotNil(t, resp.Error)
	assert.Equal(t, userclient.ErrCodeNotFound, resp.Error.Code)
}

func TestHandler_HandleGetByEmail_Success(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	want := sampleDomainUser()
	svc := &mockUserService{
		getUserByEmail: func(_ context.Context, _ string) (domain.User, error) {
			return want, nil
		},
	}
	newTestHandler(t, nc, svc)

	var resp userclient.GetUserByEmailResponse
	doRequest(t, nc, userclient.SubjectGetUserByEmail, userclient.GetUserByEmailRequest{Email: "test@example.com"}, &resp)

	require.Nil(t, resp.Error)
	require.NotNil(t, resp.User)
	assert.Equal(t, want.Email, resp.User.Email)
}

func TestHandler_HandleGetByEmail_NotFound(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	svc := &mockUserService{
		getUserByEmail: func(_ context.Context, _ string) (domain.User, error) {
			return domain.User{}, domain.ErrUserNotFound
		},
	}
	newTestHandler(t, nc, svc)

	var resp userclient.GetUserByEmailResponse
	doRequest(t, nc, userclient.SubjectGetUserByEmail, userclient.GetUserByEmailRequest{Email: "missing@example.com"}, &resp)

	require.NotNil(t, resp.Error)
	assert.Equal(t, userclient.ErrCodeNotFound, resp.Error.Code)
}

func TestHandler_HandleList_Success(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	users := []domain.User{sampleDomainUser(), sampleDomainUser()}
	svc := &mockUserService{
		listUsers: func(_ context.Context, _ *domain.UserStatus, _, _ int32) ([]domain.User, error) {
			return users, nil
		},
	}
	newTestHandler(t, nc, svc)

	var resp userclient.ListUsersResponse
	doRequest(t, nc, userclient.SubjectListUsers, userclient.ListUsersRequest{Limit: 10, Offset: 0}, &resp)

	require.Nil(t, resp.Error)
	assert.Len(t, resp.Users, 2)
}

func TestHandler_HandleUpdate_Success(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	want := sampleDomainUser()
	svc := &mockUserService{
		updateUser: func(_ context.Context, u domain.User) (domain.User, error) {
			return want, nil
		},
	}
	newTestHandler(t, nc, svc)

	var resp userclient.UpdateUserResponse
	doRequest(t, nc, userclient.SubjectUpdateUser, userclient.UpdateUserRequest{
		UserID:    testUUID,
		FirstName: "Test",
		LastName:  "User",
		Email:     "test@example.com",
		Status:    "ACTIVE",
	}, &resp)

	require.Nil(t, resp.Error)
	require.NotNil(t, resp.User)
	assert.Equal(t, want.Email, resp.User.Email)
}

func TestHandler_HandleUpdate_NotFound(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	svc := &mockUserService{
		updateUser: func(_ context.Context, _ domain.User) (domain.User, error) {
			return domain.User{}, domain.ErrUserNotFound
		},
	}
	newTestHandler(t, nc, svc)

	var resp userclient.UpdateUserResponse
	doRequest(t, nc, userclient.SubjectUpdateUser, userclient.UpdateUserRequest{
		UserID:    testUUID,
		FirstName: "Test",
		LastName:  "User",
		Email:     "test@example.com",
		Status:    "ACTIVE",
	}, &resp)

	require.NotNil(t, resp.Error)
	assert.Equal(t, userclient.ErrCodeNotFound, resp.Error.Code)
}

func TestHandler_HandleDelete_Success(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	svc := &mockUserService{
		deleteUser: func(_ context.Context, _ uuid.UUID) error { return nil },
	}
	newTestHandler(t, nc, svc)

	var resp userclient.DeleteUserResponse
	doRequest(t, nc, userclient.SubjectDeleteUser, userclient.DeleteUserRequest{UserID: testUUID}, &resp)

	assert.Nil(t, resp.Error)
}

func TestHandler_HandleDelete_NotFound(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	svc := &mockUserService{
		deleteUser: func(_ context.Context, _ uuid.UUID) error {
			return domain.ErrUserNotFound
		},
	}
	newTestHandler(t, nc, svc)

	var resp userclient.DeleteUserResponse
	doRequest(t, nc, userclient.SubjectDeleteUser, userclient.DeleteUserRequest{UserID: testUUID}, &resp)

	require.NotNil(t, resp.Error)
	assert.Equal(t, userclient.ErrCodeNotFound, resp.Error.Code)
}

func TestHandler_HandleUpdate_InvalidUUID(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	newTestHandler(t, nc, &mockUserService{})

	var resp userclient.UpdateUserResponse
	doRequest(t, nc, userclient.SubjectUpdateUser, userclient.UpdateUserRequest{
		UserID:    "not-a-uuid",
		FirstName: "Test", LastName: "User",
		Email:  "test@example.com",
		Status: "ACTIVE",
	}, &resp)

	require.NotNil(t, resp.Error)
	assert.Equal(t, userclient.ErrCodeValidation, resp.Error.Code)
}

func TestHandler_HandleUpdate_PublishesUpdatedEvent(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	eventCh := make(chan userclient.UserUpdatedEvent, 1)
	_, err = nc.Subscribe(userclient.SubjectUserUpdated, func(msg *nats.Msg) {
		var evt userclient.UserUpdatedEvent
		if json.Unmarshal(msg.Data, &evt) == nil {
			eventCh <- evt
		}
	})
	require.NoError(t, err)
	require.NoError(t, nc.Flush())

	want := sampleDomainUser()
	svc := &mockUserService{
		updateUser: func(_ context.Context, u domain.User) (domain.User, error) {
			return want, nil
		},
	}
	newTestHandler(t, nc, svc)

	var resp userclient.UpdateUserResponse
	doRequest(t, nc, userclient.SubjectUpdateUser, userclient.UpdateUserRequest{
		UserID:    testUUID,
		FirstName: "Test", LastName: "User",
		Email:  "test@example.com",
		Status: "ACTIVE",
	}, &resp)
	require.Nil(t, resp.Error)

	select {
	case evt := <-eventCh:
		assert.Equal(t, want.Email, evt.User.Email)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for updated event")
	}
}

func TestHandler_HandleDelete_InvalidUUID(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	newTestHandler(t, nc, &mockUserService{})

	var resp userclient.DeleteUserResponse
	doRequest(t, nc, userclient.SubjectDeleteUser, userclient.DeleteUserRequest{UserID: "not-a-uuid"}, &resp)

	require.NotNil(t, resp.Error)
	assert.Equal(t, userclient.ErrCodeValidation, resp.Error.Code)
}

func TestHandler_HandleDelete_PublishesDeletedEvent(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	eventCh := make(chan userclient.UserDeletedEvent, 1)
	_, err = nc.Subscribe(userclient.SubjectUserDeleted, func(msg *nats.Msg) {
		var evt userclient.UserDeletedEvent
		if json.Unmarshal(msg.Data, &evt) == nil {
			eventCh <- evt
		}
	})
	require.NoError(t, err)
	require.NoError(t, nc.Flush())

	svc := &mockUserService{
		deleteUser: func(_ context.Context, _ uuid.UUID) error { return nil },
	}
	newTestHandler(t, nc, svc)

	var resp userclient.DeleteUserResponse
	doRequest(t, nc, userclient.SubjectDeleteUser, userclient.DeleteUserRequest{UserID: testUUID}, &resp)
	assert.Nil(t, resp.Error)

	select {
	case evt := <-eventCh:
		assert.Equal(t, testUUID, evt.UserID)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for deleted event")
	}
}

func TestHandler_HandleGetByID_InvalidUUID(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	newTestHandler(t, nc, &mockUserService{})

	msg, err := nc.RequestWithContext(context.Background(), userclient.SubjectGetUserByID,
		[]byte(`{"user_id":"not-a-real-uuid-at-all-xxxx"}`))
	require.NoError(t, err)
	var resp userclient.GetUserByIDResponse
	require.NoError(t, json.Unmarshal(msg.Data, &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, userclient.ErrCodeValidation, resp.Error.Code)
}

func TestHandler_HandleList_WithStatusFilter(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	status := "ACTIVE"
	var capturedStatus *domain.UserStatus
	svc := &mockUserService{
		listUsers: func(_ context.Context, s *domain.UserStatus, _, _ int32) ([]domain.User, error) {
			capturedStatus = s
			return []domain.User{sampleDomainUser()}, nil
		},
	}
	newTestHandler(t, nc, svc)

	var resp userclient.ListUsersResponse
	doRequest(t, nc, userclient.SubjectListUsers, userclient.ListUsersRequest{
		Status: &status,
		Limit:  5,
		Offset: 0,
	}, &resp)

	require.Nil(t, resp.Error)
	require.NotNil(t, capturedStatus)
	assert.Equal(t, domain.UserStatus("ACTIVE"), *capturedStatus)
}

func TestHandler_HandleCreate_ServiceError_Internal(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	svc := &mockUserService{
		createUser: func(_ context.Context, _, _, _ string, _ *string, _ *int, _ domain.UserStatus) (domain.User, error) {
			return domain.User{}, errors.New("unexpected db error")
		},
	}
	newTestHandler(t, nc, svc)

	var resp userclient.CreateUserResponse
	doRequest(t, nc, userclient.SubjectCreateUser, userclient.CreateUserRequest{
		FirstName: "Test", LastName: "User", Email: "test@example.com",
	}, &resp)

	require.NotNil(t, resp.Error)
	assert.Equal(t, userclient.ErrCodeInternal, resp.Error.Code)
}

func TestHandler_HandleCreate_ServiceError_InvalidStatus(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	svc := &mockUserService{
		createUser: func(_ context.Context, _, _, _ string, _ *string, _ *int, _ domain.UserStatus) (domain.User, error) {
			return domain.User{}, domain.ErrInvalidStatus
		},
	}
	newTestHandler(t, nc, svc)

	var resp userclient.CreateUserResponse
	doRequest(t, nc, userclient.SubjectCreateUser, userclient.CreateUserRequest{
		FirstName: "Test", LastName: "User", Email: "test@example.com",
	}, &resp)

	require.NotNil(t, resp.Error)
	assert.Equal(t, userclient.ErrCodeValidation, resp.Error.Code)
}

func TestHandler_Stop(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	h := newTestHandler(t, nc, &mockUserService{})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_, err = nc.RequestWithContext(ctx, userclient.SubjectCreateUser, []byte(`{}`))
	cancel()
	require.NoError(t, err, "expected handler to respond before Stop")

	h.Stop()

	ctx2, cancel2 := context.WithTimeout(context.Background(), 300*time.Millisecond)
	_, err = nc.RequestWithContext(ctx2, userclient.SubjectCreateUser, []byte(`{}`))
	cancel2()
	assert.Error(t, err, "expected timeout after Stop since subscription was removed")
}

func TestHandler_Subscribe_AllSubjectsRegistered(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	newTestHandler(t, nc, &mockUserService{})

	subjects := []string{
		userclient.SubjectCreateUser,
		userclient.SubjectGetUserByID,
		userclient.SubjectGetUserByEmail,
		userclient.SubjectListUsers,
		userclient.SubjectUpdateUser,
		userclient.SubjectDeleteUser,
	}

	for _, subj := range subjects {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, reqErr := nc.RequestWithContext(ctx, subj, []byte(`{}`))
		cancel()
		assert.NoError(t, reqErr, "expected subscription on subject %s", subj)
	}
}
