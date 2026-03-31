package userclient

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrValidation    = errors.New("validation error")
	ErrInternal      = errors.New("internal error")
)

func mapRPCError(e *RPCError) error {
	switch e.Code {
	case ErrCodeNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, e.Message)
	case ErrCodeAlreadyExists:
		return fmt.Errorf("%w: %s", ErrAlreadyExists, e.Message)
	case ErrCodeValidation:
		return fmt.Errorf("%w: %s", ErrValidation, e.Message)
	default:
		return fmt.Errorf("%w: %s", ErrInternal, e.Message)
	}
}
