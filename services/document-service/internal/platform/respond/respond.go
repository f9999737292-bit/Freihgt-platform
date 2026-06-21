package respond

import (
	"encoding/json"
	"errors"
	"net/http"

	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
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
	case apperrors.CodeValidation:
		status = http.StatusBadRequest
	case apperrors.CodeNotFound:
		status = http.StatusNotFound
	case apperrors.CodeConflict:
		status = http.StatusConflict
	}
	JSON(w, status, errorBody{Error: errorPayload{
		Code: string(appErr.Code), Message: appErr.Message, Details: appErr.Details,
	}})
}
