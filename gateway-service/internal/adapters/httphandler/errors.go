package httphandler

import (
	"errors"
	"net/http"

	"gateway-service/internal/dto"
	"user-service/pkg/userclient"
)

func mapClientError(err error) (int, dto.ErrorResponse) {
	switch {
	case errors.Is(err, userclient.ErrNotFound):
		return http.StatusNotFound, errorResponse("NOT_FOUND", err.Error())
	case errors.Is(err, userclient.ErrAlreadyExists):
		return http.StatusConflict, errorResponse("ALREADY_EXISTS", err.Error())
	case errors.Is(err, userclient.ErrValidation):
		return http.StatusBadRequest, errorResponse("VALIDATION_ERROR", err.Error())
	default:
		return http.StatusInternalServerError, errorResponse("INTERNAL_ERROR", "an internal error occurred")
	}
}

func errorResponse(code, message string) dto.ErrorResponse {
	return dto.ErrorResponse{
		Error: dto.ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
}
