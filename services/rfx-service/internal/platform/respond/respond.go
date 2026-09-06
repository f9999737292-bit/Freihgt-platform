package respond

import (
	"encoding/json"
	"errors"
	"net/http"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type errorBody struct {
	Error errorPayload `json:"error,omitempty"`
}

type validationFailedBody struct {
	Code   string                          `json:"code"`
	Errors []apperrors.ValidationErrorItem `json:"errors"`
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
	case apperrors.CodeValidationFailed:
		errors := appErr.Errors
		if errors == nil {
			errors = []apperrors.ValidationErrorItem{}
		}
		JSON(w, http.StatusUnprocessableEntity, validationFailedBody{
			Code:   string(appErr.Code),
			Errors: errors,
		})
		return
	case apperrors.CodeNotFound:
		status = http.StatusNotFound
	case apperrors.CodeConflict:
		status = http.StatusConflict
	case apperrors.CodeUnauthorized:
		status = http.StatusUnauthorized
	case apperrors.CodeForbidden:
		status = http.StatusForbidden
	}
	JSON(w, status, errorBody{Error: errorPayload{
		Code: string(appErr.Code), Message: appErr.Message, Details: appErr.Details,
	}})
}
