package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/analytics-service/internal/kpi"
	apperrors "github.com/freight-platform/analytics-service/internal/platform/errors"
	"github.com/freight-platform/analytics-service/internal/platform/metrics"
	"github.com/freight-platform/analytics-service/internal/platform/respond"
	"github.com/freight-platform/analytics-service/internal/source"
)

type FoundationSource interface {
	Fetch(ctx context.Context, tenantID string) (kpi.Snapshot, error)
	Ready(ctx context.Context) error
}

type Handler struct {
	log    *slog.Logger
	source FoundationSource
	now    func() time.Time
	stats  *metrics.Metrics
}

func NewHandler(log *slog.Logger, source FoundationSource, stats *metrics.Metrics) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{log: log, source: source, now: time.Now, stats: stats}
}

func (h *Handler) GetKPI(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	kpiID := chi.URLParam(r, "kpiId")
	result := "ok"
	defer func() {
		h.stats.Observe("get_kpi", result, kpiID, started)
	}()
	if len(r.URL.Query()) > 0 {
		result = "validation"
		respond.Error(w, apperrors.Validation("analytics filters are not supported"))
		return
	}
	if !kpi.Known(kpiID) {
		result = "not_found"
		respond.Error(w, apperrors.NotFound("kpi is not available"))
		return
	}
	tenantID := tenantFrom(r.Context())
	if tenantID == "" {
		result = "unauthorized"
		respond.Error(w, apperrors.Unauthorized("tenant context is required"))
		return
	}
	snap, err := h.source.Fetch(r.Context(), tenantID)
	if err != nil {
		result = "unavailable"
		var sourceErr *source.Error
		if errors.As(err, &sourceErr) {
			h.stats.SourceError(sourceErr.Reason)
		} else {
			h.stats.SourceError("other")
		}
		h.log.Error("operations foundation source failed", slog.String("reason", result))
		respond.Error(w, apperrors.Unavailable("operations analytics source is unavailable"))
		return
	}
	if kpi.CarrierKPI(kpiID) {
		body, err := kpi.BuildCarrier(kpiID, snap, h.now())
		if err != nil {
			result = "unavailable"
			h.stats.SourceError("inconsistent")
			respond.Error(w, apperrors.Unavailable("operations analytics source is unavailable"))
			return
		}
		respond.JSON(w, http.StatusOK, body)
		return
	}
	body, err := kpi.Build(kpiID, snap, h.now())
	if err != nil {
		result = "unavailable"
		h.stats.SourceError("inconsistent")
		respond.Error(w, apperrors.Unavailable("operations analytics source is unavailable"))
		return
	}
	respond.JSON(w, http.StatusOK, body)
}
