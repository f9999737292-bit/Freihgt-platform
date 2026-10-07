package respond

import (
	"encoding/json"
	"errors"
	"net/http"

	apperrors "github.com/freight-platform/analytics-service/internal/platform/errors"
)

func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func Error(w http.ResponseWriter, err error) {
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		appErr = apperrors.Unavailable("analytics request failed")
	}
	JSON(w, appErr.Status(), map[string]any{
		"error": map[string]any{
			"code":    appErr.Code,
			"message": appErr.Message,
			"details": map[string]any{},
		},
	})
}
