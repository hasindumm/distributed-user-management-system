package userclientadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"gateway-service/internal/adapters/userclientadapter"
	"gateway-service/internal/dto"
	"user-service/pkg/userclient"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	defer l.Close()                     //nolint:errcheck
	return l.Addr().(*net.TCPAddr).Port //nolint:errcheck
}

func startNATSServer(t *testing.T) (string, func()) {
	t.Helper()
	port := freePort(t)
	opts := &natsserver.Options{Port: port, NoLog: true, NoSigs: true}
	srv, err := natsserver.NewServer(opts)
	if err != nil {
		t.Fatalf("failed to create NATS server: %v", err)
	}
	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server did not become ready")
	}
	return fmt.Sprintf("nats://127.0.0.1:%d", port), func() { srv.Shutdown() }
}

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func stubResponder(t *testing.T, nc *nats.Conn, subject string, payload any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal stub response: %v", err)
	}
	_, err = nc.Subscribe(subject, func(msg *nats.Msg) {
		if err := msg.Respond(data); err != nil {
			t.Logf("stub responder failed to respond: %v", err)
		}
	})
	if err != nil {
		t.Fatalf("failed to subscribe stub: %v", err)
	}
	if err := nc.Flush(); err != nil {
		t.Fatalf("failed to flush: %v", err)
	}
}

var sampleUserDTO = userclient.UserDTO{
	UserID:    "a7f5b6e2-1234-4567-89ab-cdef01234567",
	FirstName: "Test",
	LastName:  "User",
	Email:     "test@example.com",
	Status:    "ACTIVE",
	CreatedAt: time.Now().Format(time.RFC3339),
	UpdatedAt: time.Now().Format(time.RFC3339),
}

func newAdapter(t *testing.T, nc *nats.Conn) *userclientadapter.Adapter {
	t.Helper()
	client := userclient.NewWithConn(nc, 3*time.Second, noopLogger())
	return userclientadapter.NewUserClientAdapter(client)
}

func TestAdapter_CreateUser_Success(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer nc.Close()

	stubResponder(t, nc, userclient.SubjectCreateUser, userclient.CreateUserResponse{User: &sampleUserDTO})
	adapter := newAdapter(t, nc)

	got, err := adapter.CreateUser(context.Background(), dto.CreateUserRequest{
		FirstName: "Test", LastName: "User", Email: "test@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Email != sampleUserDTO.Email {
		t.Errorf("expected email %s, got %s", sampleUserDTO.Email, got.Email)
	}
}

func TestAdapter_CreateUser_Error(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer nc.Close()

	stubResponder(t, nc, userclient.SubjectCreateUser, userclient.CreateUserResponse{
		Error: &userclient.RPCError{Code: userclient.ErrCodeAlreadyExists, Message: "already exists"},
	})
	adapter := newAdapter(t, nc)

	_, err = adapter.CreateUser(context.Background(), dto.CreateUserRequest{
		FirstName: "Test", LastName: "User", Email: "dup@example.com",
	})
	if err == nil {
		t.Fatal("expected error but got nil")
	}
}

func TestAdapter_GetUserByID_Success(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer nc.Close()

	stubResponder(t, nc, userclient.SubjectGetUserByID, userclient.GetUserByIDResponse{User: &sampleUserDTO})
	adapter := newAdapter(t, nc)

	got, err := adapter.GetUserByID(context.Background(), sampleUserDTO.UserID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != sampleUserDTO.UserID {
		t.Errorf("expected userID %s, got %s", sampleUserDTO.UserID, got.UserID)
	}
}

func TestAdapter_GetUserByID_NotFound(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer nc.Close()

	stubResponder(t, nc, userclient.SubjectGetUserByID, userclient.GetUserByIDResponse{
		Error: &userclient.RPCError{Code: userclient.ErrCodeNotFound, Message: "not found"},
	})
	adapter := newAdapter(t, nc)

	_, err = adapter.GetUserByID(context.Background(), sampleUserDTO.UserID)
	if err == nil {
		t.Fatal("expected error but got nil")
	}
}

func TestAdapter_ListUsers_Success(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer nc.Close()

	stubResponder(t, nc, userclient.SubjectListUsers, userclient.ListUsersResponse{
		Users: []userclient.UserDTO{sampleUserDTO, sampleUserDTO},
	})
	adapter := newAdapter(t, nc)

	got, err := adapter.ListUsers(context.Background(), dto.ListUsersRequest{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 users, got %d", len(got))
	}
}

func TestAdapter_UpdateUser_Success(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer nc.Close()

	updated := sampleUserDTO
	updated.Status = "INACTIVE"
	stubResponder(t, nc, userclient.SubjectUpdateUser, userclient.UpdateUserResponse{User: &updated})
	adapter := newAdapter(t, nc)

	got, err := adapter.UpdateUser(context.Background(), sampleUserDTO.UserID, dto.UpdateUserRequest{
		FirstName: "Test", LastName: "User", Email: "test@example.com", Status: "INACTIVE",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != "INACTIVE" {
		t.Errorf("expected status INACTIVE, got %s", got.Status)
	}
}

func TestAdapter_DeleteUser_Success(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer nc.Close()

	stubResponder(t, nc, userclient.SubjectDeleteUser, userclient.DeleteUserResponse{})
	adapter := newAdapter(t, nc)

	if err := adapter.DeleteUser(context.Background(), sampleUserDTO.UserID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdapter_Subscribe_Success(t *testing.T) {
	url, shutdown := startNATSServer(t)
	defer shutdown()
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer nc.Close()

	adapter := newAdapter(t, nc)

	eventCh := make(chan userclient.Event, 10)

	sub, err := adapter.Subscribe(eventCh)
	if err != nil {
		t.Fatalf("unexpected error subscribing: %v", err)
	}
	if sub == nil {
		t.Fatal("expected non-nil subscription")
	}
	if err := sub.Unsubscribe(); err != nil {
		t.Fatalf("unexpected error unsubscribing: %v", err)
	}

	// Publish events and verify they arrive on the channel
	eventCh2 := make(chan userclient.Event, 10)
	sub2, err := adapter.Subscribe(eventCh2)
	if err != nil {
		t.Fatalf("unexpected error on second subscribe: %v", err)
	}
	defer sub2.Unsubscribe() //nolint:errcheck

	// publish a created event
	createdEvt := userclient.UserCreatedEvent{User: sampleUserDTO}
	data, _ := json.Marshal(createdEvt) //nolint:errcheck
	if err := nc.Publish(userclient.SubjectUserCreated, data); err != nil {
		t.Fatalf("failed to publish created event: %v", err)
	}

	deletedEvt := userclient.UserDeletedEvent{UserID: sampleUserDTO.UserID}
	data, _ = json.Marshal(deletedEvt) //nolint:errcheck
	if err := nc.Publish(userclient.SubjectUserDeleted, data); err != nil {
		t.Fatalf("failed to publish deleted event: %v", err)
	}

	received := map[string]bool{"created": false, "deleted": false}
	for i := 0; i < 2; i++ {
		select {
		case evt := <-eventCh2:
			switch evt.Type {
			case "created":
				e, ok := evt.Payload.(userclient.UserCreatedEvent)
				if !ok {
					t.Fatal("expected UserCreatedEvent payload")
				}
				if e.User.Email != sampleUserDTO.Email {
					t.Errorf("expected email %s, got %s", sampleUserDTO.Email, e.User.Email)
				}
				received["created"] = true
			case "deleted":
				e, ok := evt.Payload.(userclient.UserDeletedEvent)
				if !ok {
					t.Fatal("expected UserDeletedEvent payload")
				}
				if e.UserID != sampleUserDTO.UserID {
					t.Errorf("expected userID %s, got %s", sampleUserDTO.UserID, e.UserID)
				}
				received["deleted"] = true
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for event")
		}
	}
	if !received["created"] {
		t.Error("did not receive created event")
	}
	if !received["deleted"] {
		t.Error("did not receive deleted event")
	}
}
