package ports

import "gateway-service/internal/dto"

type Subscription interface {
	Unsubscribe() error
}

type EventHandlers struct {
	OnCreated func(UserCreatedEvent)
	OnUpdated func(UserUpdatedEvent)
	OnDeleted func(UserDeletedEvent)
}

type UserCreatedEvent struct {
	User dto.UserResponse
}

type UserUpdatedEvent struct {
	User dto.UserResponse
}
type UserDeletedEvent struct {
	UserID string
}
