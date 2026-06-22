package respond

import (
	"encoding/json"
	"errors"
	"net/http"

	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
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
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func Error(w http.ResponseWriter, err error) {
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		appErr = apperrors.Internal("unexpected error", err)
	}

	status := http.StatusInternalServerError
	switch appErr.Code {
	case apperrors.CodeValidation, apperrors.CodeTenantRequired,
		apperrors.CodeEntityTypeInvalid, apperrors.CodeEntityIDInvalid,
		apperrors.CodeFieldInvalidType, apperrors.CodeValidationRuleFailed,
		apperrors.CodeSystemFieldProtected, apperrors.CodeTenantMismatch,
		apperrors.CodeFormTemplateNotPublished, apperrors.CodeFormTemplateNotDraft:
		status = http.StatusBadRequest
	case apperrors.CodeFormTemplateConflict:
		status = http.StatusConflict
	case apperrors.CodeNotFound, apperrors.CodeFormTemplateNotFound, apperrors.CodeFieldNotFound:
		status = http.StatusNotFound
	}

	JSON(w, status, errorBody{
		Error: errorPayload{
			Code:    string(appErr.Code),
			Message: appErr.Message,
			Details: appErr.Details,
		},
	})
}
