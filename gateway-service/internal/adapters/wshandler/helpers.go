package wshandler

import (
	"errors"
	"time"

	"user-service/pkg/userclient"
)

const rpcTimeout = 10 * time.Second

func errorResponse(msg Message, code, message string) Response {
	return Response{
		Action:    msg.Action,
		RequestID: msg.RequestID,
		Success:   false,
		Error:     &WSError{Code: code, Message: message},
	}
}

func mapServiceError(msg Message, err error) Response {
	switch {
	case errors.Is(err, userclient.ErrNotFound):
		return errorResponse(msg, "NOT_FOUND", err.Error())
	case errors.Is(err, userclient.ErrAlreadyExists):
		return errorResponse(msg, "ALREADY_EXISTS", err.Error())
	case errors.Is(err, userclient.ErrValidation):
		return errorResponse(msg, "VALIDATION_ERROR", err.Error())
	default:
		return errorResponse(msg, "INTERNAL_ERROR", "an internal error occurred")
	}
}
