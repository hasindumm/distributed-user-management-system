package userclient_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"
	"user-service/pkg/userclient"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func startServer(t *testing.T) (string, func()) {
	t.Helper()

	port, err := freePort()
	require.NoError(t, err)

	opts := &natsserver.Options{
		Port:   port,
		NoLog:  true,
		NoSigs: true,
	}
	srv, err := natsserver.NewServer(opts)
	require.NoError(t, err)

	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server did not become ready")
	}

	return fmt.Sprintf("nats://127.0.0.1:%d", port), func() { srv.Shutdown() }
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close() //nolint:errcheck
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address type")
	}
	return addr.Port, nil
}

// stubResponder subscribes to a NATS subject and replies with the given payload.
func stubResponder(t *testing.T, nc *nats.Conn, subject string, payload any) {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	_, err = nc.Subscribe(subject, func(msg *nats.Msg) {
		require.NoError(t, msg.Respond(data))
	})
	require.NoError(t, err)
	require.NoError(t, nc.Flush())
}

// New / NewWithConn

func TestNew_ValidURL(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	client, err := userclient.New(userclient.Config{NATSURL: url}, testLogger)
	require.NoError(t, err)
	defer client.Close()
}

func TestNew_InvalidURL(t *testing.T) {
	_, err := userclient.New(userclient.Config{NATSURL: "nats://127.0.0.1:1"}, testLogger)
	require.Error(t, err)
}

func TestNew_DefaultTimeout(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	// Timeout=0 should default to 5s without error
	client, err := userclient.New(userclient.Config{NATSURL: url, Timeout: 0}, testLogger)
	require.NoError(t, err)
	defer client.Close()
}

// CreateUser

func TestClient_CreateUser_Success(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	phone := "+1234567890"
	age := 25
	want := userclient.UserDTO{
		UserID:    "a7f5b6e2-1234-4567-89ab-cdef01234567",
		FirstName: "Test",
		LastName:  "User",
		Email:     "test@example.com",
		Phone:     &phone,
		Age:       &age,
		Status:    "ACTIVE",
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	stubResponder(t, nc, userclient.SubjectCreateUser, userclient.CreateUserResponse{User: &want})

	client := userclient.NewWithConn(nc, 3*time.Second, testLogger)

	got, err := client.CreateUser(context.Background(), userclient.CreateUserRequest{
		FirstName: want.FirstName,
		LastName:  want.LastName,
		Email:     want.Email,
	})
	require.NoError(t, err)
	assert.Equal(t, want.UserID, got.UserID)
	assert.Equal(t, want.Email, got.Email)
}

func TestClient_CreateUser_RPCError(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	stubResponder(t, nc, userclient.SubjectCreateUser, userclient.CreateUserResponse{
		Error: &userclient.RPCError{Code: userclient.ErrCodeAlreadyExists, Message: "email already exists"},
	})

	client := userclient.NewWithConn(nc, 3*time.Second, testLogger)

	_, err = client.CreateUser(context.Background(), userclient.CreateUserRequest{
		FirstName: "A", LastName: "B", Email: "dup@example.com",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, userclient.ErrAlreadyExists)
}

// GetUserByID

func TestClient_GetUserByID_NotFound(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	stubResponder(t, nc, userclient.SubjectGetUserByID, userclient.GetUserByIDResponse{
		Error: &userclient.RPCError{Code: userclient.ErrCodeNotFound, Message: "not found"},
	})

	client := userclient.NewWithConn(nc, 3*time.Second, testLogger)

	_, err = client.GetUserByID(context.Background(), "a7f5b6e2-1234-4567-89ab-cdef01234567")
	require.Error(t, err)
	assert.ErrorIs(t, err, userclient.ErrNotFound)
}

// GetUserByEmail

func TestClient_GetUserByEmail_Success(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	want := userclient.UserDTO{UserID: "abc123", Email: "found@example.com", Status: "ACTIVE",
		CreatedAt: time.Now().Format(time.RFC3339), UpdatedAt: time.Now().Format(time.RFC3339)}
	stubResponder(t, nc, userclient.SubjectGetUserByEmail, userclient.GetUserByEmailResponse{User: &want})

	client := userclient.NewWithConn(nc, 3*time.Second, testLogger)

	got, err := client.GetUserByEmail(context.Background(), "found@example.com")
	require.NoError(t, err)
	assert.Equal(t, want.Email, got.Email)
}

// ListUsers

func TestClient_ListUsers_Success(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	stubResponder(t, nc, userclient.SubjectListUsers, userclient.ListUsersResponse{
		Users: []userclient.UserDTO{
			{UserID: "1", Email: "a@example.com", Status: "ACTIVE",
				CreatedAt: time.Now().Format(time.RFC3339), UpdatedAt: time.Now().Format(time.RFC3339)},
			{UserID: "2", Email: "b@example.com", Status: "INACTIVE",
				CreatedAt: time.Now().Format(time.RFC3339), UpdatedAt: time.Now().Format(time.RFC3339)},
		},
	})

	client := userclient.NewWithConn(nc, 3*time.Second, testLogger)

	got, err := client.ListUsers(context.Background(), userclient.ListUsersRequest{Limit: 10, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

// UpdateUser

func TestClient_UpdateUser_Success(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	want := userclient.UserDTO{UserID: "abc123", Email: "updated@example.com", Status: "INACTIVE",
		CreatedAt: time.Now().Format(time.RFC3339), UpdatedAt: time.Now().Format(time.RFC3339)}
	stubResponder(t, nc, userclient.SubjectUpdateUser, userclient.UpdateUserResponse{User: &want})

	client := userclient.NewWithConn(nc, 3*time.Second, testLogger)

	got, err := client.UpdateUser(context.Background(), userclient.UpdateUserRequest{
		UserID: "abc123", FirstName: "Up", LastName: "Dated",
		Email: "updated@example.com", Status: "INACTIVE",
	})
	require.NoError(t, err)
	assert.Equal(t, want.Status, got.Status)
}

// DeleteUser

func TestClient_DeleteUser_Success(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	stubResponder(t, nc, userclient.SubjectDeleteUser, userclient.DeleteUserResponse{})

	client := userclient.NewWithConn(nc, 3*time.Second, testLogger)

	err = client.DeleteUser(context.Background(), "a7f5b6e2-1234-4567-89ab-cdef01234567")
	require.NoError(t, err)
}

func TestClient_DeleteUser_InternalError(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	stubResponder(t, nc, userclient.SubjectDeleteUser, userclient.DeleteUserResponse{
		Error: &userclient.RPCError{Code: userclient.ErrCodeInternal, Message: "something went wrong"},
	})

	client := userclient.NewWithConn(nc, 3*time.Second, testLogger)

	err = client.DeleteUser(context.Background(), "a7f5b6e2-1234-4567-89ab-cdef01234567")
	require.Error(t, err)
	assert.ErrorIs(t, err, userclient.ErrInternal)
}

//mapRPCError coverage: ErrCodeValidation path

func TestClient_ValidationError(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	stubResponder(t, nc, userclient.SubjectGetUserByID, userclient.GetUserByIDResponse{
		Error: &userclient.RPCError{Code: userclient.ErrCodeValidation, Message: "invalid uuid"},
	})

	client := userclient.NewWithConn(nc, 3*time.Second, testLogger)

	_, err = client.GetUserByID(context.Background(), "a7f5b6e2-1234-4567-89ab-cdef01234567")
	require.Error(t, err)
	assert.ErrorIs(t, err, userclient.ErrValidation)
}
