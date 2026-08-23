package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	"github.com/freight-platform/freight-cost-service/internal/platform/respond"
	"github.com/freight-platform/freight-cost-service/internal/repository"
	"github.com/freight-platform/freight-cost-service/internal/service"
	"github.com/freight-platform/freight-cost-service/internal/worker"
)

type AnalyticsProjectionHandler struct {
	analytics *service.AnalyticsProjectionService
	worker    *worker.AnalyticsProjectionWorker
	state     *repository.AnalyticsProjectionStateRepository
}
func NewAnalyticsProjectionHandler(
	analytics *service.AnalyticsProjectionService,
	worker *worker.AnalyticsProjectionWorker,
	state *repository.AnalyticsProjectionStateRepository,
) *AnalyticsProjectionHandler {
	return &AnalyticsProjectionHandler{
		analytics: analytics,
		worker:    worker,
		state:     state,
	}
}

func (h *AnalyticsProjectionHandler) RebuildTenant(w http.ResponseWriter, r *http.Request) {
	if h.worker == nil {
		respond.Error(w, apperrors.NotFound("analytics projection worker is not enabled"))
		return
	}
	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid tenant id", map[string]any{"field": "tenantId"}))
		return
	}
	if err := h.worker.RebuildTenantNow(r.Context(), tenantID); err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusAccepted, map[string]any{
		"tenant_id": tenantID.String(),
		"status":    "rebuild_completed",
	})
}

func (h *AnalyticsProjectionHandler) GetTenantState(w http.ResponseWriter, r *http.Request) {
	if h.state == nil {
		respond.Error(w, apperrors.NotFound("analytics projection state is not available"))
		return
	}
	tenantID, err := uuid.Parse(chi.URLParam(r, "tenantId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid tenant id", map[string]any{"field": "tenantId"}))
		return
	}
	state, err := h.state.Get(r.Context(), nil, "cost_analytics_period_projection", tenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"projection_name":        state.ProjectionName,
		"tenant_id":              state.TenantID.String(),
		"projection_version":     state.ProjectionVersion,
		"source_watermark":       state.SourceWatermark,
		"last_successful_run_at": state.LastSuccessfulRunAt,
		"calculated_at":          state.CalculatedAt,
		"data_through":           state.DataThrough,
		"status":                 state.Status,
		"last_error_code":        state.LastErrorCode,
		"last_error_message":     state.LastErrorMessage,
		"updated_at":             state.UpdatedAt,
	})
}
