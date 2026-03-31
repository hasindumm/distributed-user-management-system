package userclient_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
	"user-service/pkg/userclient"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeUser(id, email string) userclient.UserDTO {
	return userclient.UserDTO{
		UserID:    id,
		Email:     email,
		Status:    "ACTIVE",
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
}

func newCachedClient(t *testing.T, nc *nats.Conn, url string, seedUsers []userclient.UserDTO) *userclient.Client {
	t.Helper()
	stubResponder(t, nc, userclient.SubjectListUsers, userclient.ListUsersResponse{Users: seedUsers})
	client, err := userclient.New(userclient.Config{
		NATSURL:      url,
		Timeout:      3 * time.Second,
		CacheEnabled: true,
	}, testLogger)
	require.NoError(t, err)
	return client
}

func publishEvent(t *testing.T, nc *nats.Conn, subject string, payload any) {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, nc.Publish(subject, data))
	require.NoError(t, nc.Flush())
}

func TestCache_StartupLoad_FillsCache(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	user := makeUser("startup-user-1", "startup@example.com")
	client := newCachedClient(t, nc, url, []userclient.UserDTO{user})
	defer client.Close()

	got, err := client.GetUserByID(context.Background(), user.UserID)
	require.NoError(t, err)
	assert.Equal(t, user.UserID, got.UserID)
	assert.Equal(t, user.Email, got.Email)
}

func TestCache_Hit_GetByID(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	user := makeUser("hit-user", "hit@example.com")
	client := newCachedClient(t, nc, url, []userclient.UserDTO{user})
	defer client.Close()

	got, err := client.GetUserByID(context.Background(), user.UserID)
	require.NoError(t, err)
	assert.Equal(t, user.UserID, got.UserID)
}

func TestCache_Hit_GetByEmail(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	user := makeUser("email-hit-user", "emailhit@example.com")
	client := newCachedClient(t, nc, url, []userclient.UserDTO{user})
	defer client.Close()

	got, err := client.GetUserByEmail(context.Background(), user.Email)
	require.NoError(t, err)
	assert.Equal(t, user.Email, got.Email)
}

func TestCache_Hit_List(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	users := []userclient.UserDTO{
		makeUser("list-user-1", "list1@example.com"),
		makeUser("list-user-2", "list2@example.com"),
	}
	client := newCachedClient(t, nc, url, users)
	defer client.Close()

	got, err := client.ListUsers(context.Background(), userclient.ListUsersRequest{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestCache_Miss_FallsThroughToRPC(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	client := newCachedClient(t, nc, url, nil)
	defer client.Close()

	rpcUser := makeUser("rpc-user", "rpc@example.com")
	stubResponder(t, nc, userclient.SubjectGetUserByID, userclient.GetUserByIDResponse{User: &rpcUser})

	got, err := client.GetUserByID(context.Background(), rpcUser.UserID)
	require.NoError(t, err)
	assert.Equal(t, rpcUser.UserID, got.UserID)
}

func TestCache_Disabled_AlwaysUsesRPC(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	user := makeUser("no-cache-user", "nocache@example.com")
	stubResponder(t, nc, userclient.SubjectGetUserByID, userclient.GetUserByIDResponse{User: &user})

	client := userclient.NewWithConn(nc, 3*time.Second, testLogger)

	got, err := client.GetUserByID(context.Background(), user.UserID)
	require.NoError(t, err)
	assert.Equal(t, user.UserID, got.UserID)
}

func TestCache_OnCreatedEvent_AddsToCache(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	client := newCachedClient(t, nc, url, nil)
	defer client.Close()

	newUser := makeUser("created-event-user", "created@example.com")
	publishEvent(t, nc, userclient.SubjectUserCreated, userclient.UserCreatedEvent{User: newUser})

	time.Sleep(50 * time.Millisecond)

	got, err := client.GetUserByID(context.Background(), newUser.UserID)
	require.NoError(t, err)
	assert.Equal(t, newUser.UserID, got.UserID)
}

func TestCache_OnUpdatedEvent_UpdatesCache(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	original := makeUser("update-event-user", "original@example.com")
	client := newCachedClient(t, nc, url, []userclient.UserDTO{original})
	defer client.Close()

	updated := original
	updated.Email = "updated@example.com"
	updated.Status = "INACTIVE"
	publishEvent(t, nc, userclient.SubjectUserUpdated, userclient.UserUpdatedEvent{User: updated})

	time.Sleep(50 * time.Millisecond)

	got, err := client.GetUserByID(context.Background(), updated.UserID)
	require.NoError(t, err)
	assert.Equal(t, "updated@example.com", got.Email)
	assert.Equal(t, "INACTIVE", got.Status)
}

func TestCache_OnDeletedEvent_RemovesFromCache(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	user := makeUser("delete-event-user", "delete@example.com")
	client := newCachedClient(t, nc, url, []userclient.UserDTO{user})
	defer client.Close()

	publishEvent(t, nc, userclient.SubjectUserDeleted, userclient.UserDeletedEvent{UserID: user.UserID})

	time.Sleep(50 * time.Millisecond)

	stubResponder(t, nc, userclient.SubjectGetUserByID, userclient.GetUserByIDResponse{
		Error: &userclient.RPCError{Code: userclient.ErrCodeNotFound, Message: "not found"},
	})

	_, err = client.GetUserByID(context.Background(), user.UserID)
	require.Error(t, err)
	assert.ErrorIs(t, err, userclient.ErrNotFound)
}

func TestCache_ConcurrentReads_NoDataRace(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	const userCount = 10
	users := make([]userclient.UserDTO, userCount)
	for i := range users {
		users[i] = makeUser(fmt.Sprintf("concurrent-user-%d", i), fmt.Sprintf("user%d@example.com", i))
	}
	client := newCachedClient(t, nc, url, users)
	defer client.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			userID := fmt.Sprintf("concurrent-user-%d", idx%userCount)
			got, getErr := client.GetUserByID(context.Background(), userID)
			assert.NoError(t, getErr)
			assert.Equal(t, userID, got.UserID)
		}(i)
	}
	wg.Wait()
}
