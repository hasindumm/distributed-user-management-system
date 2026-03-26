package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	UserId    uuid.UUID
	FirstName string
	LastName  string
	Email     string
	Phone     *string
	Age       *int
	Status    UserStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type UserStatus string

const (
	UserStatusActive    UserStatus = "ACTIVE"
	UserStatusInactive  UserStatus = "INACTIVE"
	UserStatusSuspended UserStatus = "SUSPENDED"
)

func (u UserStatus) IsValid() bool {
	if u == UserStatusSuspended || u == UserStatusActive || u == UserStatusInactive {
		return true
	}
	return false
}
