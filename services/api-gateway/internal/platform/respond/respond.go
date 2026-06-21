package respond

import (
	"encoding/json"
	"errors"
	"net/http"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
)

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func Error(w http.ResponseWriter, err error) {
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		appErr = apperrors.Internal("unexpected error", err)
	}

	status := http.StatusInternalServerError
	switch appErr.Code {
	case apperrors.CodeRouteNotFound:
		status = http.StatusNotFound
	case apperrors.CodeUnauthorized:
		status = http.StatusUnauthorized
	case apperrors.CodeForbidden:
		status = http.StatusForbidden
	case apperrors.CodeServiceUnavailable:
		status = http.StatusBadGateway
	case apperrors.CodeRateLimitExceeded:
		status = http.StatusTooManyRequests
	case apperrors.CodeRequestBodyTooLarge:
		status = http.StatusRequestEntityTooLarge
	}

	JSON(w, status, errorBody{Error: errorPayload{
		Code: string(appErr.Code), Message: appErr.Message, Details: appErr.Details,
	}})
}
