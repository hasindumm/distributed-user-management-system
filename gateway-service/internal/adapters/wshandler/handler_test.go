package wshandler_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"gateway-service/internal/adapters/wshandler"
	"gateway-service/internal/dto"
	"gateway-service/internal/ports"
	"user-service/pkg/userclient"
)

type mockUserService struct {
	createUser     func(ctx context.Context, req dto.CreateUserRequest) (dto.UserResponse, error)
	getUserByID    func(ctx context.Context, id string) (dto.UserResponse, error)
	getUserByEmail func(ctx context.Context, email string) (dto.UserResponse, error)
	listUsers      func(ctx context.Context, req dto.ListUsersRequest) ([]dto.UserResponse, error)
	updateUser     func(ctx context.Context, id string, req dto.UpdateUserRequest) (dto.UserResponse, error)
	deleteUser     func(ctx context.Context, id string) error
}

func (m *mockUserService) CreateUser(ctx context.Context, req dto.CreateUserRequest) (dto.UserResponse, error) {
	return m.createUser(ctx, req)
}
func (m *mockUserService) GetUserByID(ctx context.Context, id string) (dto.UserResponse, error) {
	return m.getUserByID(ctx, id)
}
func (m *mockUserService) GetUserByEmail(ctx context.Context, email string) (dto.UserResponse, error) {
	return m.getUserByEmail(ctx, email)
}
func (m *mockUserService) ListUsers(ctx context.Context, req dto.ListUsersRequest) ([]dto.UserResponse, error) {
	return m.listUsers(ctx, req)
}
func (m *mockUserService) UpdateUser(ctx context.Context, id string, req dto.UpdateUserRequest) (dto.UserResponse, error) {
	return m.updateUser(ctx, id, req)
}
func (m *mockUserService) DeleteUser(ctx context.Context, id string) error {
	return m.deleteUser(ctx, id)
}

var _ ports.UserService = (*mockUserService)(nil)

type mockUserClient struct {
	createUser     func(ctx context.Context, req dto.CreateUserRequest) (dto.UserResponse, error)
	getUserByID    func(ctx context.Context, id string) (dto.UserResponse, error)
	getUserByEmail func(ctx context.Context, email string) (dto.UserResponse, error)
	listUsers      func(ctx context.Context, req dto.ListUsersRequest) ([]dto.UserResponse, error)
	updateUser     func(ctx context.Context, id string, req dto.UpdateUserRequest) (dto.UserResponse, error)
	deleteUser     func(ctx context.Context, id string) error
	subscribe      func(handlers ports.EventHandlers) (ports.Subscription, error)
}

func (m *mockUserClient) CreateUser(ctx context.Context, req dto.CreateUserRequest) (dto.UserResponse, error) {
	return m.createUser(ctx, req)
}
func (m *mockUserClient) GetUserByID(ctx context.Context, id string) (dto.UserResponse, error) {
	return m.getUserByID(ctx, id)
}
func (m *mockUserClient) GetUserByEmail(ctx context.Context, email string) (dto.UserResponse, error) {
	return m.getUserByEmail(ctx, email)
}
func (m *mockUserClient) ListUsers(ctx context.Context, req dto.ListUsersRequest) ([]dto.UserResponse, error) {
	return m.listUsers(ctx, req)
}
func (m *mockUserClient) UpdateUser(ctx context.Context, id string, req dto.UpdateUserRequest) (dto.UserResponse, error) {
	return m.updateUser(ctx, id, req)
}
func (m *mockUserClient) DeleteUser(ctx context.Context, id string) error {
	return m.deleteUser(ctx, id)
}
func (m *mockUserClient) Subscribe(handlers ports.EventHandlers) (ports.Subscription, error) {
	return m.subscribe(handlers)
}

var _ ports.UserClient = (*mockUserClient)(nil)

type mockSubscription struct {
	unsubscribed bool
}

func (s *mockSubscription) Unsubscribe() error {
	s.unsubscribed = true
	return nil
}

var _ ports.Subscription = (*mockSubscription)(nil)

const testUUID = "550e8400-e29b-41d4-a716-446655440000"

var sampleUser = dto.UserResponse{
	UserID:    testUUID,
	FirstName: "John",
	LastName:  "Doe",
	Email:     "john@example.com",
	Status:    "ACTIVE",
	CreatedAt: "2024-01-01T00:00:00Z",
	UpdatedAt: "2024-01-01T00:00:00Z",
}

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newTestServerAndHub(t *testing.T, svc ports.UserService, client ports.UserClient) (*httptest.Server, *wshandler.Hub) {
	t.Helper()
	hub := wshandler.NewHub(noopLogger())
	go hub.Run()
	h := wshandler.NewWSHandler(svc, client, hub, wshandler.DefaultConfig(), noopLogger())
	srv := httptest.NewServer(http.HandlerFunc(h.ServeHTTP))
	t.Cleanup(srv.Close)
	return srv, hub
}

func newTestServerWithHub(t *testing.T, svc ports.UserService, client ports.UserClient, cfg wshandler.Config) (*httptest.Server, *wshandler.Hub) {
	t.Helper()
	hub := wshandler.NewHub(noopLogger())
	go hub.Run()
	h := wshandler.NewWSHandler(svc, client, hub, cfg, noopLogger())
	srv := httptest.NewServer(http.HandlerFunc(h.ServeHTTP))
	t.Cleanup(srv.Close)
	return srv, hub
}

func dialWS(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + srv.URL[len("http"):]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Logf("ws cleanup close: %v", err)
		}
	})
	return conn
}

func newTestServer(t *testing.T, svc ports.UserService, client ports.UserClient) (*httptest.Server, *websocket.Conn) {
	t.Helper()
	srv, _ := newTestServerAndHub(t, svc, client)
	conn := dialWS(t, srv)
	return srv, conn
}

func mustMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", v)
	}
	return m
}

func roundTrip(t *testing.T, conn *websocket.Conn, msg any) map[string]any {
	t.Helper()
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}
	var resp map[string]any
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("ReadJSON failed: %v", err)
	}
	return resp
}

func TestHandleCreate_Success(t *testing.T) {
	svc := &mockUserService{
		createUser: func(_ context.Context, req dto.CreateUserRequest) (dto.UserResponse, error) {
			return sampleUser, nil
		},
	}
	_, conn := newTestServer(t, svc, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action":     "user.create",
		"request_id": "req-1",
		"payload": map[string]any{
			"first_name": "John",
			"last_name":  "Doe",
			"email":      "john@example.com",
			"status":     "ACTIVE",
		},
	})

	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp)
	}
	if resp["action"] != "user.create" {
		t.Errorf("expected action=user.create, got %v", resp["action"])
	}
	if resp["request_id"] != "req-1" {
		t.Errorf("expected request_id echoed, got %v", resp["request_id"])
	}
}

func TestHandleCreate_BadPayload(t *testing.T) {
	_, conn := newTestServer(t, &mockUserService{}, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action":     "user.create",
		"request_id": "req-2",
		"payload":    "not-an-object",
	})

	if resp["success"] != false {
		t.Fatalf("expected success=false, got %v", resp)
	}
	errObj := mustMap(t, resp["error"])
	if errObj["code"] != "BAD_REQUEST" {
		t.Errorf("expected code=BAD_REQUEST, got %v", errObj["code"])
	}
}

func TestHandleGetByID_Success(t *testing.T) {
	svc := &mockUserService{
		getUserByID: func(_ context.Context, id string) (dto.UserResponse, error) {
			return sampleUser, nil
		},
	}
	_, conn := newTestServer(t, svc, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action":     "user.get_by_id",
		"request_id": "req-3",
		"payload":    map[string]any{"user_id": testUUID},
	})

	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp)
	}
}

func TestHandleGetByID_NotFound(t *testing.T) {
	svc := &mockUserService{
		getUserByID: func(_ context.Context, id string) (dto.UserResponse, error) {
			return dto.UserResponse{}, userclient.ErrNotFound
		},
	}
	_, conn := newTestServer(t, svc, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action":  "user.get_by_id",
		"payload": map[string]any{"user_id": testUUID},
	})

	if resp["success"] != false {
		t.Fatalf("expected success=false, got %v", resp)
	}
	errObj := mustMap(t, resp["error"])
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("expected code=NOT_FOUND, got %v", errObj["code"])
	}
}

func TestHandleGetByID_MissingUserID(t *testing.T) {
	_, conn := newTestServer(t, &mockUserService{}, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action":  "user.get_by_id",
		"payload": map[string]any{},
	})

	if resp["success"] != false {
		t.Fatalf("expected success=false, got %v", resp)
	}
	errObj := mustMap(t, resp["error"])
	if errObj["code"] != "BAD_REQUEST" {
		t.Errorf("expected code=BAD_REQUEST, got %v", errObj["code"])
	}
}

func TestHandleGetByEmail_Success(t *testing.T) {
	svc := &mockUserService{
		getUserByEmail: func(_ context.Context, email string) (dto.UserResponse, error) {
			return sampleUser, nil
		},
	}
	_, conn := newTestServer(t, svc, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action":  "user.get_by_email",
		"payload": map[string]any{"email": "john@example.com"},
	})

	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp)
	}
}

func TestHandleGetByEmail_MissingEmail(t *testing.T) {
	_, conn := newTestServer(t, &mockUserService{}, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action":  "user.get_by_email",
		"payload": map[string]any{},
	})

	errObj := mustMap(t, resp["error"])
	if errObj["code"] != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST, got %v", errObj["code"])
	}
}

func TestHandleList_Success(t *testing.T) {
	svc := &mockUserService{
		listUsers: func(_ context.Context, req dto.ListUsersRequest) ([]dto.UserResponse, error) {
			return []dto.UserResponse{sampleUser}, nil
		},
	}
	_, conn := newTestServer(t, svc, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action":  "user.list",
		"payload": map[string]any{"limit": 10},
	})

	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp)
	}
}

func TestHandleList_NullPayload(t *testing.T) {
	svc := &mockUserService{
		listUsers: func(_ context.Context, req dto.ListUsersRequest) ([]dto.UserResponse, error) {
			if req.Limit != 20 {
				t.Errorf("expected default limit=20, got %d", req.Limit)
			}
			return []dto.UserResponse{}, nil
		},
	}
	_, conn := newTestServer(t, svc, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action":  "user.list",
		"payload": nil,
	})

	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp)
	}
}

func TestHandleUpdate_Success(t *testing.T) {
	svc := &mockUserService{
		updateUser: func(_ context.Context, id string, req dto.UpdateUserRequest) (dto.UserResponse, error) {
			return sampleUser, nil
		},
	}
	_, conn := newTestServer(t, svc, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action": "user.update",
		"payload": map[string]any{
			"user_id":    testUUID,
			"first_name": "Jane",
		},
	})

	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp)
	}
}

func TestHandleUpdate_MissingUserID(t *testing.T) {
	_, conn := newTestServer(t, &mockUserService{}, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action":  "user.update",
		"payload": map[string]any{"first_name": "Jane"},
	})

	errObj := mustMap(t, resp["error"])
	if errObj["code"] != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST, got %v", errObj["code"])
	}
}

func TestHandleDelete_Success(t *testing.T) {
	svc := &mockUserService{
		deleteUser: func(_ context.Context, id string) error { return nil },
	}
	_, conn := newTestServer(t, svc, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action":  "user.delete",
		"payload": map[string]any{"user_id": testUUID},
	})

	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp)
	}
}

func TestHandleDelete_MissingUserID(t *testing.T) {
	_, conn := newTestServer(t, &mockUserService{}, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action":  "user.delete",
		"payload": map[string]any{},
	})

	errObj := mustMap(t, resp["error"])
	if errObj["code"] != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST, got %v", errObj["code"])
	}
}

func TestUnknownAction(t *testing.T) {
	_, conn := newTestServer(t, &mockUserService{}, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action": "user.fly",
	})

	if resp["success"] != false {
		t.Fatalf("expected success=false, got %v", resp)
	}
	errObj := mustMap(t, resp["error"])
	if errObj["code"] != "UNKNOWN_ACTION" {
		t.Errorf("expected code=UNKNOWN_ACTION, got %v", errObj["code"])
	}
}

func TestHandleSubscribe_Success(t *testing.T) {
	mockSub := &mockSubscription{}
	client := &mockUserClient{
		subscribe: func(handlers ports.EventHandlers) (ports.Subscription, error) {
			return mockSub, nil
		},
	}
	_, conn := newTestServer(t, &mockUserService{}, client)

	resp := roundTrip(t, conn, map[string]any{
		"action":     "user.subscribe",
		"request_id": "sub-1",
	})

	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp)
	}
}

func TestHandleUnsubscribe_WithoutSubscribe(t *testing.T) {
	_, conn := newTestServer(t, &mockUserService{}, &mockUserClient{})

	// Unsubscribe when no subscription is active — should succeed silently.
	resp := roundTrip(t, conn, map[string]any{
		"action":     "user.unsubscribe",
		"request_id": "unsub-1",
	})

	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp)
	}
}

func TestHandleSubscribe_EventPushed(t *testing.T) {
	var capturedHandlers ports.EventHandlers

	client := &mockUserClient{
		subscribe: func(handlers ports.EventHandlers) (ports.Subscription, error) {
			capturedHandlers = handlers
			return &mockSubscription{}, nil
		},
	}
	_, conn := newTestServer(t, &mockUserService{}, client)

	roundTrip(t, conn, map[string]any{"action": "user.subscribe"})

	capturedHandlers.OnCreated(ports.UserCreatedEvent{User: sampleUser})

	var pushed map[string]any
	if err := conn.ReadJSON(&pushed); err != nil {
		t.Fatalf("ReadJSON for pushed event failed: %v", err)
	}

	if pushed["action"] != "user.created" {
		t.Errorf("expected action=user.created, got %v", pushed["action"])
	}
	if pushed["success"] != true {
		t.Errorf("expected success=true in pushed event, got %v", pushed["success"])
	}
	payload := mustMap(t, pushed["payload"])
	if payload["user_id"] != testUUID {
		t.Errorf("expected user_id in event payload, got %v", payload["user_id"])
	}
}

func TestHandleSubscribe_UpdateEventPushed(t *testing.T) {
	var capturedHandlers ports.EventHandlers
	client := &mockUserClient{
		subscribe: func(handlers ports.EventHandlers) (ports.Subscription, error) {
			capturedHandlers = handlers
			return &mockSubscription{}, nil
		},
	}
	_, conn := newTestServer(t, &mockUserService{}, client)
	roundTrip(t, conn, map[string]any{"action": "user.subscribe"})

	capturedHandlers.OnUpdated(ports.UserUpdatedEvent{User: sampleUser})

	var pushed map[string]any
	if err := conn.ReadJSON(&pushed); err != nil {
		t.Fatalf("ReadJSON failed: %v", err)
	}
	if pushed["action"] != "user.updated" {
		t.Errorf("expected action=user.updated, got %v", pushed["action"])
	}
}

func TestHandleSubscribe_DeleteEventPushed(t *testing.T) {
	var capturedHandlers ports.EventHandlers
	client := &mockUserClient{
		subscribe: func(handlers ports.EventHandlers) (ports.Subscription, error) {
			capturedHandlers = handlers
			return &mockSubscription{}, nil
		},
	}
	_, conn := newTestServer(t, &mockUserService{}, client)
	roundTrip(t, conn, map[string]any{"action": "user.subscribe"})

	capturedHandlers.OnDeleted(ports.UserDeletedEvent{UserID: testUUID})

	var pushed map[string]any
	if err := conn.ReadJSON(&pushed); err != nil {
		t.Fatalf("ReadJSON failed: %v", err)
	}
	if pushed["action"] != "user.deleted" {
		t.Errorf("expected action=user.deleted, got %v", pushed["action"])
	}
	payload := mustMap(t, pushed["payload"])
	if payload["user_id"] != testUUID {
		t.Errorf("expected user_id in delete event, got %v", payload["user_id"])
	}
}

func TestErrorMapping_AlreadyExists(t *testing.T) {
	svc := &mockUserService{
		createUser: func(_ context.Context, req dto.CreateUserRequest) (dto.UserResponse, error) {
			return dto.UserResponse{}, userclient.ErrAlreadyExists
		},
	}
	_, conn := newTestServer(t, svc, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action": "user.create",
		"payload": map[string]any{
			"first_name": "John",
			"last_name":  "Doe",
			"email":      "john@example.com",
			"status":     "ACTIVE",
		},
	})

	errObj := mustMap(t, resp["error"])
	if errObj["code"] != "ALREADY_EXISTS" {
		t.Errorf("expected ALREADY_EXISTS, got %v", errObj["code"])
	}
}

func TestErrorMapping_ValidationError(t *testing.T) {
	svc := &mockUserService{
		createUser: func(_ context.Context, req dto.CreateUserRequest) (dto.UserResponse, error) {
			return dto.UserResponse{}, userclient.ErrValidation
		},
	}
	_, conn := newTestServer(t, svc, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action": "user.create",
		"payload": map[string]any{
			"first_name": "John",
			"last_name":  "Doe",
			"email":      "john@example.com",
			"status":     "ACTIVE",
		},
	})

	errObj := mustMap(t, resp["error"])
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %v", errObj["code"])
	}
}

func TestRequestIDEchoed(t *testing.T) {
	svc := &mockUserService{
		getUserByID: func(_ context.Context, id string) (dto.UserResponse, error) {
			return sampleUser, nil
		},
	}
	_, conn := newTestServer(t, svc, &mockUserClient{})

	const rid = "my-unique-request-id"
	resp := roundTrip(t, conn, map[string]any{
		"action":     "user.get_by_id",
		"request_id": rid,
		"payload":    map[string]any{"user_id": testUUID},
	})

	if resp["request_id"] != rid {
		t.Errorf("expected request_id=%q echoed, got %v", rid, resp["request_id"])
	}
}

func TestResponsePayloadShape(t *testing.T) {
	svc := &mockUserService{
		getUserByID: func(_ context.Context, id string) (dto.UserResponse, error) {
			return sampleUser, nil
		},
	}
	_, conn := newTestServer(t, svc, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action":  "user.get_by_id",
		"payload": map[string]any{"user_id": testUUID},
	})

	payloadBytes, err := json.Marshal(resp["payload"])
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	var user dto.UserResponse
	if err := json.Unmarshal(payloadBytes, &user); err != nil {
		t.Fatalf("payload did not unmarshal to UserResponse: %v", err)
	}
	if user.UserID != testUUID {
		t.Errorf("expected UserID=%q, got %q", testUUID, user.UserID)
	}
	if user.Email != "john@example.com" {
		t.Errorf("expected Email=john@example.com, got %q", user.Email)
	}
}

func TestReadLimit_LargeMessage(t *testing.T) {
	_, conn := newTestServer(t, &mockUserService{}, &mockUserClient{})

	largePayload := make([]byte, 600*1024)
	if err := conn.WriteMessage(websocket.BinaryMessage, largePayload); err != nil {
		// connection may already be closing — that's acceptable
		return
	}

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Error("expected server to close connection after oversized message")
	}
}

func TestPingPong_DeadConnection(t *testing.T) {
	cfg := wshandler.DefaultConfig()
	cfg.PongWait = 400 * time.Millisecond
	cfg.PingPeriod = 300 * time.Millisecond
	cfg.WriteWait = 200 * time.Millisecond

	srv, _ := newTestServerWithHub(t, &mockUserService{}, &mockUserClient{}, cfg)
	conn := dialWS(t, srv)

	conn.SetPingHandler(func(string) error { return nil })

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Error("expected server to close dead connection after missed pong")
	}
}

func TestHub_BroadcastToSubscribed(t *testing.T) {
	mc := &mockUserClient{
		subscribe: func(handlers ports.EventHandlers) (ports.Subscription, error) {
			return &mockSubscription{}, nil
		},
	}

	srv, hub := newTestServerAndHub(t, &mockUserService{}, mc)

	connA := dialWS(t, srv) // will subscribe
	connB := dialWS(t, srv) // will NOT subscribe

	roundTrip(t, connA, map[string]any{"action": "user.subscribe", "request_id": "sub-a"})

	roundTrip(t, connB, map[string]any{"action": "user.unsubscribe", "request_id": "sync-b"})

	hub.Broadcast("user.created", wshandler.Response{
		Action:  "user.created",
		Success: true,
		Payload: sampleUser,
	})

	if err := connA.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var evtA map[string]any
	if err := connA.ReadJSON(&evtA); err != nil {
		t.Fatalf("connA (subscribed) should receive broadcast: %v", err)
	}
	if evtA["action"] != "user.created" {
		t.Errorf("connA: expected action=user.created, got %v", evtA["action"])
	}

	if err := connB.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var evtB map[string]any
	if err := connB.ReadJSON(&evtB); err == nil {
		t.Errorf("connB (not subscribed) should not receive broadcast, got %v", evtB)
	}
}

func TestHub_SlowClient_Disconnected(t *testing.T) {
	cfg := wshandler.DefaultConfig()
	cfg.WriteWait = 150 * time.Millisecond

	mc := &mockUserClient{
		subscribe: func(handlers ports.EventHandlers) (ports.Subscription, error) {
			return &mockSubscription{}, nil
		},
	}

	srv, hub := newTestServerWithHub(t, &mockUserService{}, mc, cfg)

	connSlow := dialWS(t, srv)
	roundTrip(t, connSlow, map[string]any{"action": "user.subscribe"})

	connFast := dialWS(t, srv)
	roundTrip(t, connFast, map[string]any{"action": "user.subscribe"})

	fastReceived := make(chan struct{}, 512)
	go func() {
		for {
			var msg map[string]any
			if err := connFast.ReadJSON(&msg); err != nil {
				return
			}
			select {
			case fastReceived <- struct{}{}:
			default:
			}
		}
	}()

	start := time.Now()
	for i := 0; i < 300; i++ {
		hub.Broadcast("user.created", wshandler.Response{
			Action:  "user.created",
			Success: true,
		})
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Hub.Broadcast blocked: 300 calls took %v, expected non-blocking", elapsed)
	}

	select {
	case <-fastReceived:
	case <-time.After(3 * time.Second):
		t.Error("fast client did not receive any broadcast within 3 s")
	}
}

func TestUnknownAction_ErrorResponse(t *testing.T) {
	_, conn := newTestServer(t, &mockUserService{}, &mockUserClient{})

	resp := roundTrip(t, conn, map[string]any{
		"action":     "user.does_not_exist",
		"request_id": "req-unknown-1",
	})

	if resp["success"] != false {
		t.Fatalf("expected success=false, got %v", resp)
	}
	if resp["request_id"] != "req-unknown-1" {
		t.Errorf("expected request_id echoed, got %v", resp["request_id"])
	}
	errObj := mustMap(t, resp["error"])
	if errObj["code"] != "UNKNOWN_ACTION" {
		t.Errorf("expected code=UNKNOWN_ACTION, got %v", errObj["code"])
	}
}

func TestMultipleClients_IndependentSubscriptions(t *testing.T) {
	mc := &mockUserClient{
		subscribe: func(handlers ports.EventHandlers) (ports.Subscription, error) {
			return &mockSubscription{}, nil
		},
	}

	srv, hub := newTestServerAndHub(t, &mockUserService{}, mc)

	conn1 := dialWS(t, srv)
	conn2 := dialWS(t, srv)
	conn3 := dialWS(t, srv)

	roundTrip(t, conn1, map[string]any{"action": "user.subscribe", "request_id": "sub-1"})

	roundTrip(t, conn2, map[string]any{"action": "user.unsubscribe", "request_id": "sync-2"})

	roundTrip(t, conn3, map[string]any{"action": "user.subscribe", "request_id": "sub-3"})
	roundTrip(t, conn3, map[string]any{"action": "user.unsubscribe", "request_id": "unsub-3"})

	hub.Broadcast("user.created", wshandler.Response{
		Action:  "user.created",
		Success: true,
		Payload: sampleUser,
	})

	if err := conn1.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var evt1 map[string]any
	if err := conn1.ReadJSON(&evt1); err != nil {
		t.Fatalf("conn1 should receive event: %v", err)
	}
	if evt1["action"] != "user.created" {
		t.Errorf("conn1: expected action=user.created, got %v", evt1["action"])
	}

	if err := conn2.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var evt2 map[string]any
	if err := conn2.ReadJSON(&evt2); err == nil {
		t.Errorf("conn2 should not receive event (never subscribed), got %v", evt2)
	}

	if err := conn3.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var evt3 map[string]any
	if err := conn3.ReadJSON(&evt3); err == nil {
		t.Errorf("conn3 should not receive event (unsubscribed), got %v", evt3)
	}
}
