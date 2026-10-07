package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
)

const AuthorizedAnalyticsCaller = "analytics-service"

// RequireAnalyticsCaller rejects any internal caller other than analytics-service.
// The shared internal token middleware must already have accepted the request.
func RequireAnalyticsCaller(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(r.Header.Get(HeaderInternalServiceName)) != AuthorizedAnalyticsCaller {
			respond.Error(w, apperrors.Forbidden("caller is not allowed to read analytics source facts"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type OperationsAnalyticsSourceReader interface {
	OperationsFoundation(ctx context.Context, tenantID uuid.UUID) (domain.OperationsAnalyticsSourceSnapshot, error)
}

type OperationsAnalyticsSourceHandler struct {
	reader OperationsAnalyticsSourceReader
}

func NewOperationsAnalyticsSourceHandler(reader OperationsAnalyticsSourceReader) *OperationsAnalyticsSourceHandler {
	return &OperationsAnalyticsSourceHandler{reader: reader}
}

func (h *OperationsAnalyticsSourceHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.reader == nil {
		respond.Error(w, apperrors.Internal("analytics source is not configured", nil))
		return
	}
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	snap, err := h.reader.OperationsFoundation(r.Context(), tenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, snap)
}
