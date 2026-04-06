package userclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
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

	time.Sleep(200 * time.Millisecond)

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

	time.Sleep(200 * time.Millisecond)

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

	// register the RPC stub before polling so every cache-miss falls through to RPC
	stubResponder(t, nc, userclient.SubjectGetUserByID, userclient.GetUserByIDResponse{
		Error: &userclient.RPCError{Code: userclient.ErrCodeNotFound, Message: "not found"},
	})

	require.Eventually(t, func() bool {
		_, getErr := client.GetUserByID(context.Background(), user.UserID)
		return errors.Is(getErr, userclient.ErrNotFound)
	}, 500*time.Millisecond, 10*time.Millisecond)
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

func TestCache_LazyLoad_GetByID(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	client := newCachedClient(t, nc, url, nil) // start with empty cache
	defer client.Close()

	rpcUser := makeUser("lazy-id-user", "lazyid@example.com")

	var rpcCalls int
	var mu sync.Mutex
	sub, err := nc.Subscribe(userclient.SubjectGetUserByID, func(msg *nats.Msg) {
		mu.Lock()
		rpcCalls++
		mu.Unlock()
		data, marshalErr := json.Marshal(userclient.GetUserByIDResponse{User: &rpcUser})
		require.NoError(t, marshalErr)
		require.NoError(t, msg.Respond(data))
	})
	require.NoError(t, err)
	defer sub.Unsubscribe() //nolint:errcheck
	require.NoError(t, nc.Flush())

	got, err := client.GetUserByID(context.Background(), rpcUser.UserID)
	require.NoError(t, err)
	assert.Equal(t, rpcUser.UserID, got.UserID)

	got2, err := client.GetUserByID(context.Background(), rpcUser.UserID)
	require.NoError(t, err)
	assert.Equal(t, rpcUser.UserID, got2.UserID)

	mu.Lock()
	assert.Equal(t, 1, rpcCalls, "RPC should be called exactly once; second request must hit cache")
	mu.Unlock()
}

func TestCache_LazyLoad_GetByEmail(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	client := newCachedClient(t, nc, url, nil)
	defer client.Close()

	rpcUser := makeUser("lazy-email-user", "lazyemail@example.com")

	var rpcCalls int
	var mu sync.Mutex
	sub, err := nc.Subscribe(userclient.SubjectGetUserByEmail, func(msg *nats.Msg) {
		mu.Lock()
		rpcCalls++
		mu.Unlock()
		data, marshalErr := json.Marshal(userclient.GetUserByEmailResponse{User: &rpcUser})
		require.NoError(t, marshalErr)
		require.NoError(t, msg.Respond(data))
	})
	require.NoError(t, err)
	defer sub.Unsubscribe() //nolint:errcheck
	require.NoError(t, nc.Flush())

	got, err := client.GetUserByEmail(context.Background(), rpcUser.Email)
	require.NoError(t, err)
	assert.Equal(t, rpcUser.Email, got.Email)

	got2, err := client.GetUserByEmail(context.Background(), rpcUser.Email)
	require.NoError(t, err)
	assert.Equal(t, rpcUser.Email, got2.Email)

	mu.Lock()
	assert.Equal(t, 1, rpcCalls, "RPC should be called exactly once; second request must hit cache")
	mu.Unlock()
}

func TestCache_PaginatedStartup(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	firstBatch := make([]userclient.UserDTO, 1000)
	for i := range firstBatch {
		firstBatch[i] = makeUser(fmt.Sprintf("paginated-%d", i), fmt.Sprintf("page%d@example.com", i))
	}
	secondBatch := make([]userclient.UserDTO, 500)
	for i := range secondBatch {
		secondBatch[i] = makeUser(fmt.Sprintf("paginated-%d", 1000+i), fmt.Sprintf("page%d@example.com", 1000+i))
	}

	sub, err := nc.Subscribe(userclient.SubjectListUsers, func(msg *nats.Msg) {
		var req userclient.ListUsersRequest
		require.NoError(t, json.Unmarshal(msg.Data, &req))

		var resp userclient.ListUsersResponse
		if req.Offset == 0 {
			resp = userclient.ListUsersResponse{Users: firstBatch}
		} else {
			resp = userclient.ListUsersResponse{Users: secondBatch}
		}
		data, marshalErr := json.Marshal(resp)
		require.NoError(t, marshalErr)
		require.NoError(t, msg.Respond(data))
	})
	require.NoError(t, err)
	defer sub.Unsubscribe() //nolint:errcheck
	require.NoError(t, nc.Flush())

	client, err := userclient.New(userclient.Config{
		NATSURL:      url,
		Timeout:      3 * time.Second,
		CacheEnabled: true,
	}, testLogger)
	require.NoError(t, err)
	defer client.Close()

	users, err := client.ListUsers(context.Background(), userclient.ListUsersRequest{Limit: 2000})
	require.NoError(t, err)
	assert.Len(t, users, 1500, "loadCache should have loaded all 1500 users across two batches")
}

func TestCache_ChannelFull_DropsWithWarning(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	client := newCachedClient(t, nc, url, nil)
	defer client.Close()

	const floodCount = 600
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < floodCount; i++ {
			u := makeUser(fmt.Sprintf("flood-user-%d", i), fmt.Sprintf("flood%d@example.com", i))
			publishEvent(t, nc, userclient.SubjectUserCreated, userclient.UserCreatedEvent{User: u})
		}
	}()

	select {
	case <-done:
		// all publishes completed — no deadlock or panic
	case <-time.After(3 * time.Second):
		t.Fatal("publishing events timed out — select/default drop guard may be missing")
	}
}

func goroutineStacks() string {
	for size := 64 * 1024; ; size *= 2 {
		buf := make([]byte, size)
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return string(buf[:n])
		}
	}
}

func TestCache_GoroutineStopsOnClose(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	stubResponder(t, nc, userclient.SubjectListUsers, userclient.ListUsersResponse{Users: nil})

	client, err := userclient.New(userclient.Config{
		NATSURL:      url,
		Timeout:      3 * time.Second,
		CacheEnabled: true,
	}, testLogger)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return strings.Contains(goroutineStacks(), "processCacheUpdates")
	}, 500*time.Millisecond, 5*time.Millisecond,
		"processCacheUpdates goroutine should be running after New()")

	client.Close()

	require.Eventually(t, func() bool {
		return !strings.Contains(goroutineStacks(), "processCacheUpdates")
	}, 500*time.Millisecond, 10*time.Millisecond,
		"processCacheUpdates goroutine should stop after Close()")
}

func TestCache_DrainOnShutdown(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	client := newCachedClient(t, nc, url, nil)

	const eventCount = 10
	for i := 0; i < eventCount; i++ {
		u := makeUser(fmt.Sprintf("drain-user-%d", i), fmt.Sprintf("drain%d@example.com", i))
		publishEvent(t, nc, userclient.SubjectUserCreated, userclient.UserCreatedEvent{User: u})
	}

	closeDone := make(chan struct{})
	go func() {
		client.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
	case <-time.After(3 * time.Second):
		t.Fatal("Close() timed out — drain goroutine may be stuck processing updateCh")
	}
}

func TestListUsers_CacheComplete_ServedFromCache(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	users := []userclient.UserDTO{
		makeUser("complete-1", "complete1@example.com"),
		makeUser("complete-2", "complete2@example.com"),
	}
	client := newCachedClient(t, nc, url, users)
	defer client.Close()

	var rpcCalls int
	var mu sync.Mutex
	sub, err := nc.Subscribe(userclient.SubjectListUsers, func(msg *nats.Msg) {
		mu.Lock()
		rpcCalls++
		mu.Unlock()
		data, marshalErr := json.Marshal(userclient.ListUsersResponse{Users: users})
		require.NoError(t, marshalErr)
		require.NoError(t, msg.Respond(data))
	})
	require.NoError(t, err)
	defer sub.Unsubscribe() //nolint:errcheck
	require.NoError(t, nc.Flush())

	got, err := client.ListUsers(context.Background(), userclient.ListUsersRequest{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, got, 2)

	mu.Lock()
	assert.Equal(t, 0, rpcCalls, "ListUsers must be served from cache when fully loaded — no RPC expected")
	mu.Unlock()
}

func TestListUsers_CacheIncomplete_FallsThroughToRPC(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	partialBatch := make([]userclient.UserDTO, 1000)
	for i := range partialBatch {
		partialBatch[i] = makeUser(
			fmt.Sprintf("incomplete-%d", i),
			fmt.Sprintf("incomplete%d@example.com", i),
		)
	}

	listSub, err := nc.Subscribe(userclient.SubjectListUsers, func(msg *nats.Msg) {
		data, marshalErr := json.Marshal(userclient.ListUsersResponse{Users: partialBatch})
		require.NoError(t, marshalErr)
		require.NoError(t, msg.Respond(data))
	})
	require.NoError(t, err)
	defer listSub.Unsubscribe() //nolint:errcheck
	require.NoError(t, nc.Flush())

	rpcUser := makeUser("rpc-list-user", "rpclist@example.com")
	stubResponder(t, nc, userclient.SubjectGetUserByID, userclient.GetUserByIDResponse{User: &rpcUser})

	noCacheClient := userclient.NewWithConn(nc, 3*time.Second, testLogger)
	var rpcListCalls int
	var mu2 sync.Mutex
	sub2, err := nc.Subscribe(userclient.SubjectListUsers+".direct", func(msg *nats.Msg) {
		mu2.Lock()
		rpcListCalls++
		mu2.Unlock()
	})
	require.NoError(t, err)
	defer sub2.Unsubscribe() //nolint:errcheck

	_, err = noCacheClient.ListUsers(context.Background(), userclient.ListUsersRequest{Limit: 10})
	require.NoError(t, err)
}

func TestListUsers_CacheDisabled_AlwaysRPC(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	wantUsers := []userclient.UserDTO{
		makeUser("disabled-1", "disabled1@example.com"),
		makeUser("disabled-2", "disabled2@example.com"),
	}

	var rpcCalls int
	var mu sync.Mutex
	sub, err := nc.Subscribe(userclient.SubjectListUsers, func(msg *nats.Msg) {
		mu.Lock()
		rpcCalls++
		mu.Unlock()
		data, marshalErr := json.Marshal(userclient.ListUsersResponse{Users: wantUsers})
		require.NoError(t, marshalErr)
		require.NoError(t, msg.Respond(data))
	})
	require.NoError(t, err)
	defer sub.Unsubscribe() //nolint:errcheck
	require.NoError(t, nc.Flush())

	client := userclient.NewWithConn(nc, 3*time.Second, testLogger)

	got, err := client.ListUsers(context.Background(), userclient.ListUsersRequest{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, got, 2)

	_, err = client.ListUsers(context.Background(), userclient.ListUsersRequest{Limit: 10})
	require.NoError(t, err)

	mu.Lock()
	assert.Equal(t, 2, rpcCalls, "cache-disabled client must call RPC every time")
	mu.Unlock()
}

func TestLoadCache_SetsFullyLoadedOnCompletion(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	seed := []userclient.UserDTO{makeUser("flag-user", "flag@example.com")}
	client := newCachedClient(t, nc, url, seed)
	defer client.Close()

	var rpcCalled bool
	sub, err := nc.Subscribe(userclient.SubjectListUsers, func(msg *nats.Msg) {
		rpcCalled = true
		data, _ := json.Marshal(userclient.ListUsersResponse{Users: seed}) //nolint:errcheck
		_ = msg.Respond(data)                                              //nolint:errcheck
	})
	require.NoError(t, err)
	defer sub.Unsubscribe() //nolint:errcheck
	require.NoError(t, nc.Flush())

	got, err := client.ListUsers(context.Background(), userclient.ListUsersRequest{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.False(t, rpcCalled,
		"cacheFullyLoaded should be true after natural loop end — ListUsers must not call RPC")
}

func TestLoadCache_DoesNotSetFullyLoadedOnTimeout(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	errSub, err := nc.Subscribe(userclient.SubjectListUsers, func(msg *nats.Msg) {
		data, _ := json.Marshal(userclient.ListUsersResponse{ //nolint:errcheck
			Error: &userclient.RPCError{Code: userclient.ErrCodeInternal, Message: "simulated failure"},
		})
		_ = msg.Respond(data) //nolint:errcheck
	})
	require.NoError(t, err)
	require.NoError(t, nc.Flush())

	client, err := userclient.New(userclient.Config{
		NATSURL:      url,
		Timeout:      3 * time.Second,
		CacheEnabled: true,
	}, testLogger)
	require.NoError(t, err)
	defer client.Close()

	require.NoError(t, errSub.Unsubscribe())

	wantUsers := []userclient.UserDTO{makeUser("timeout-user", "timeout@example.com")}
	stubResponder(t, nc, userclient.SubjectListUsers, userclient.ListUsersResponse{Users: wantUsers})

	got, err := client.ListUsers(context.Background(), userclient.ListUsersRequest{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, got, 1, "ListUsers must fall through to RPC when cache is incomplete")
	assert.Equal(t, "timeout-user", got[0].UserID)
}

func makeUsers(n int, prefix string) []userclient.UserDTO {
	users := make([]userclient.UserDTO, n)
	for i := range users {
		users[i] = makeUser(
			fmt.Sprintf("%s-%d", prefix, i),
			fmt.Sprintf("%s%d@example.com", prefix, i),
		)
	}
	return users
}

func TestListUsers_Cache_RespectsLimit(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	seed := makeUsers(100, "limit-user")
	client := newCachedClient(t, nc, url, seed)
	defer client.Close()

	got, err := client.ListUsers(context.Background(), userclient.ListUsersRequest{Limit: 10, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, got, 10, "cache must honour Limit=10 and return exactly 10 users")
}

func TestListUsers_Cache_RespectsOffset(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	seed := makeUsers(100, "offset-user")
	client := newCachedClient(t, nc, url, seed)
	defer client.Close()

	got, err := client.ListUsers(context.Background(), userclient.ListUsersRequest{Limit: 10, Offset: 20})
	require.NoError(t, err)
	assert.Len(t, got, 10, "cache must honour Offset=20 and return 10 users starting from position 20")
}

func TestListUsers_Cache_OffsetBeyondEnd(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	seed := makeUsers(100, "beyond-user")
	client := newCachedClient(t, nc, url, seed)
	defer client.Close()

	got, err := client.ListUsers(context.Background(), userclient.ListUsersRequest{Limit: 10, Offset: 200})
	require.NoError(t, err)
	assert.Empty(t, got, "offset past end of cache must return empty slice, not error")
}

func TestListUsers_Cache_LastPage(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	seed := makeUsers(100, "lastpage-user")
	client := newCachedClient(t, nc, url, seed)
	defer client.Close()

	got, err := client.ListUsers(context.Background(), userclient.ListUsersRequest{Limit: 10, Offset: 95})
	require.NoError(t, err)
	assert.Len(t, got, 5, "last page must return only the remaining users, not the full limit")
}

func TestListUsers_Cache_DefaultLimit(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	seed := makeUsers(100, "default-limit-user")
	client := newCachedClient(t, nc, url, seed)
	defer client.Close()

	got, err := client.ListUsers(context.Background(), userclient.ListUsersRequest{})
	require.NoError(t, err)
	assert.Len(t, got, 50, "Limit=0 must apply defaultListLimit (50) even when serving from cache")
}

func TestListUsers_Cache_StatusFilter_WithPagination(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	active := makeUsers(60, "active-filter-user")
	inactive := make([]userclient.UserDTO, 40)
	for i := range inactive {
		inactive[i] = makeUser(fmt.Sprintf("inactive-filter-user-%d", i), fmt.Sprintf("inactive%d@example.com", i))
		inactive[i].Status = "INACTIVE"
	}
	seed := append(active, inactive...)

	client := newCachedClient(t, nc, url, seed)
	defer client.Close()

	status := "ACTIVE"
	got, err := client.ListUsers(context.Background(), userclient.ListUsersRequest{
		Limit:  10,
		Offset: 0,
		Status: &status,
	})
	require.NoError(t, err)
	assert.Len(t, got, 10, "pagination must be applied to the status-filtered set (60 ACTIVE), not the full 100")
	for _, u := range got {
		assert.Equal(t, "ACTIVE", u.Status, "result must contain only ACTIVE users")
	}
}

func TestCacheList_Direct_Pagination(t *testing.T) {
	url, shutdown := startServer(t)
	defer shutdown()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	seed := makeUsers(10, "direct-page-user")
	client := newCachedClient(t, nc, url, seed)
	defer client.Close()

	got, err := client.ListUsers(context.Background(), userclient.ListUsersRequest{Limit: 3, Offset: 2})
	require.NoError(t, err)
	assert.Len(t, got, 3, "list with limit=3 offset=2 must return exactly 3 users")
}
