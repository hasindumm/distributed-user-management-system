package dto

type CreateUserRequest struct {
	FirstName string  `json:"first_name" validate:"required,max=100"`
	LastName  string  `json:"last_name"  validate:"required,max=100"`
	Email     string  `json:"email"      validate:"required,email,max=255"`
	Phone     *string `json:"phone,omitempty" validate:"omitempty,max=20"`
	Age       *int    `json:"age,omitempty"   validate:"omitempty,min=0,max=150"`
	Status    string  `json:"status,omitempty" validate:"omitempty,oneof=ACTIVE INACTIVE SUSPENDED"`
}

type UpdateUserRequest struct {
	FirstName string  `json:"first_name" validate:"required,max=100"`
	LastName  string  `json:"last_name"  validate:"required,max=100"`
	Email     string  `json:"email"      validate:"required,email,max=255"`
	Phone     *string `json:"phone,omitempty" validate:"omitempty,max=20"`
	Age       *int    `json:"age,omitempty"   validate:"omitempty,min=0,max=150"`
	Status    string  `json:"status"     validate:"required,oneof=ACTIVE INACTIVE SUSPENDED"`
}

type ListUsersRequest struct {
	Status *string `validate:"omitempty,oneof=ACTIVE INACTIVE SUSPENDED"`
	Limit  int32   `validate:"min=1,max=1000"`
	Offset int32   `validate:"min=0"`
}

type UserResponse struct {
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

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}
