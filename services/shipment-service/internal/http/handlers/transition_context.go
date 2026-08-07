package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

func resolveVerifiedUser(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get("X-User-ID"))
	if raw == "" {
		return uuid.Nil, apperrors.Unauthorized("user context is required")
	}
	userID, err := domain.ParseUUID(raw, "user_id")
	if err != nil {
		return uuid.Nil, err
	}
	if userID == uuid.Nil {
		return uuid.Nil, apperrors.Validation("user_id is required", map[string]any{"field": "user_id"})
	}
	return userID, nil
}

func resolveUserStatusTransitionContext(r *http.Request) (domain.StatusTransitionContext, error) {
	userID, err := resolveVerifiedUser(r)
	if err != nil {
		return domain.StatusTransitionContext{}, err
	}

	var correlationID *string
	if requestID := strings.TrimSpace(sharedmiddleware.RequestIDFromContext(r.Context())); requestID != "" {
		correlationID = &requestID
	} else if headerID := strings.TrimSpace(r.Header.Get(sharedmiddleware.RequestIDHeader)); headerID != "" {
		correlationID = &headerID
	}

	return domain.NewUserTransitionContext(userID, correlationID, time.Now().UTC()), nil
}

func withCancelReason(transition domain.StatusTransitionContext, reason string) domain.StatusTransitionContext {
	trimmed := strings.TrimSpace(reason)
	if trimmed != "" {
		transition.ReasonCode = &trimmed
	}
	return transition
}
