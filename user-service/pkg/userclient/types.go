package userclient

type ErrorCode string

const (
	ErrCodeNotFound      ErrorCode = "NOT_FOUND"
	ErrCodeAlreadyExists ErrorCode = "ALREADY_EXISTS"
	ErrCodeValidation    ErrorCode = "VALIDATION_ERROR"
	ErrCodeInternal      ErrorCode = "INTERNAL_ERROR"
)

type RPCError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

type UserDTO struct {
	UserID    string  `json:"user_id"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     string  `json:"email"`
	Phone     *string `json:"phone,omitempty"`
	Age       *int    `json:"age,omitempty"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type CreateUserRequest struct {
	FirstName string  `json:"first_name" validate:"required,max=100"`
	LastName  string  `json:"last_name"  validate:"required,max=100"`
	Email     string  `json:"email"      validate:"required,email,max=255"`
	Phone     *string `json:"phone,omitempty" validate:"omitempty,max=20"`
	Age       *int    `json:"age,omitempty"   validate:"omitempty,min=0,max=150"`
	Status    string  `json:"status,omitempty" validate:"omitempty,oneof=ACTIVE INACTIVE SUSPENDED"`
}

type CreateUserResponse struct {
	User  *UserDTO  `json:"user,omitempty"`
	Error *RPCError `json:"error,omitempty"`
}

type GetUserByIDRequest struct {
	UserID string `json:"user_id" validate:"required,uuid"`
}

type GetUserByIDResponse struct {
	User  *UserDTO  `json:"user,omitempty"`
	Error *RPCError `json:"error,omitempty"`
}

type GetUserByEmailRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type GetUserByEmailResponse struct {
	User  *UserDTO  `json:"user,omitempty"`
	Error *RPCError `json:"error,omitempty"`
}

type ListUsersRequest struct {
	Status *string `json:"status,omitempty" validate:"omitempty,oneof=ACTIVE INACTIVE SUSPENDED"`
	Limit  int32   `json:"limit"  validate:"min=1,max=1000"`
	Offset int32   `json:"offset" validate:"min=0"`
}

type ListUsersResponse struct {
	Users []UserDTO `json:"users"`
	Error *RPCError `json:"error,omitempty"`
}

type UpdateUserRequest struct {
	UserID    string  `json:"user_id"    validate:"required,uuid"`
	FirstName string  `json:"first_name" validate:"required,max=100"`
	LastName  string  `json:"last_name"  validate:"required,max=100"`
	Email     string  `json:"email"      validate:"required,email,max=255"`
	Phone     *string `json:"phone,omitempty" validate:"omitempty,max=20"`
	Age       *int    `json:"age,omitempty"   validate:"omitempty,min=0,max=150"`
	Status    string  `json:"status"     validate:"required,oneof=ACTIVE INACTIVE SUSPENDED"`
}

type UpdateUserResponse struct {
	User  *UserDTO  `json:"user,omitempty"`
	Error *RPCError `json:"error,omitempty"`
}

type DeleteUserRequest struct {
	UserID string `json:"user_id" validate:"required,uuid"`
}

type DeleteUserResponse struct {
	Error *RPCError `json:"error,omitempty"`
}

type UserCreatedEvent struct {
	User UserDTO `json:"user"`
}

type UserUpdatedEvent struct {
	User UserDTO `json:"user"`
}

type UserDeletedEvent struct {
	UserID string `json:"user_id"`
}

type ListAllUsersResponse struct {
	Users []UserDTO `json:"users"`
	Error *RPCError `json:"error,omitempty"`
}

type Event struct {
	Type    string
	Payload any
}
