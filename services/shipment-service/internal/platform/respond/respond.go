package respond

import (
	"encoding/json"
	"errors"
	"net/http"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
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
	case apperrors.CodeNotFound, apperrors.CodeRelationshipRequestNotFound:
		status = http.StatusNotFound
	case apperrors.CodeConflict, apperrors.CodeRelationshipIdempotencyConflict, apperrors.CodeRelationshipRequestPendingConflict, apperrors.CodeRelationshipRequestStale, apperrors.CodeRelationshipRequestAlreadyAccepted:
		status = http.StatusConflict
	case apperrors.CodeUnauthorized, apperrors.CodeAuthenticationRequired:
		status = http.StatusUnauthorized
	case apperrors.CodeForbidden, apperrors.CodeRelationshipRequestForbidden, apperrors.CodeRelationshipCommandsDisabled:
		status = http.StatusForbidden
	case apperrors.CodeUnavailable, apperrors.CodeRelationshipSourceUnavailable, apperrors.CodeRelationshipStoreUnavailable:
		status = http.StatusServiceUnavailable
	}
	JSON(w, status, errorBody{Error: errorPayload{
		Code: string(appErr.Code), Message: appErr.Message, Details: appErr.Details,
	}})
}
