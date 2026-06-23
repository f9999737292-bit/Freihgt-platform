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
	Preview map[string]any `json:"preview,omitempty"`
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
		apperrors.CodeSystemFieldProtected, apperrors.CodeReadOnlyFieldProtected, apperrors.CodeTenantMismatch,
		apperrors.CodeFormTemplateNotPublished, apperrors.CodeFormTemplateNotDraft:
		status = http.StatusBadRequest
	case apperrors.CodeFormTemplateConflict:
		status = http.StatusConflict
	case apperrors.CodeMigrationBlocked, apperrors.CodeMigrationWarningsRequireConfirmation,
		apperrors.CodeBatchMigrationBlocked, apperrors.CodeBatchMigrationWarningsRequireConfirmation:
		status = http.StatusConflict
	case apperrors.CodeNotFound, apperrors.CodeFormTemplateNotFound, apperrors.CodeFieldNotFound:
		status = http.StatusNotFound
	}

	JSON(w, status, errorBody{
		Error: buildErrorPayload(appErr),
	})
}

func buildErrorPayload(appErr *apperrors.AppError) errorPayload {
	details := appErr.Details
	if details == nil {
		details = map[string]any{}
	}
	var preview map[string]any
	if rawPreview, ok := details["preview"].(map[string]any); ok {
		preview = rawPreview
		delete(details, "preview")
	}
	if len(details) == 0 {
		details = map[string]any{}
	}
	return errorPayload{
		Code:    string(appErr.Code),
		Message: appErr.Message,
		Details: details,
		Preview: preview,
	}
}
