package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
)

type TrackingContextReader interface {
	GetTrackingContext(ctx context.Context, operatingTenantID, executionID, stopID uuid.UUID) (domain.TrackingStopContext, error)
}

type TrackingContextHandler struct {
	reader TrackingContextReader
}

func NewTrackingContextHandler(reader TrackingContextReader) *TrackingContextHandler {
	return &TrackingContextHandler{reader: reader}
}

func (h *TrackingContextHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.reader == nil {
		respond.Error(w, apperrors.NotFound("tracking context not found"))
		return
	}
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	executionID, err := domain.ParseUUID(chi.URLParam(r, "executionId"), "execution_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	stopID, err := domain.ParseUUID(chi.URLParam(r, "stopId"), "stop_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	view, err := h.reader.GetTrackingContext(r.Context(), tenantID, executionID, stopID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, trackingContextJSON(view))
}

func trackingContextJSON(view domain.TrackingStopContext) map[string]any {
	body := map[string]any{
		"executionId":           view.ExecutionID,
		"revisionId":            view.RevisionID,
		"executionStopId":       view.ExecutionStopID,
		"ordinal":               view.Ordinal,
		"status":                view.Status,
		"plannedArrival":        view.PlannedArrival,
		"locationId":            view.LocationID,
		"targetLatitude":        view.TargetLatitude,
		"targetLongitude":       view.TargetLongitude,
		"driverId":              view.DriverID,
		"vehicleId":             view.VehicleID,
		"pointKind":             view.PointKind,
		"stopRole":              view.StopRole,
		"liveEtaStopId":         view.LiveETAStopID,
		"liveEtaOrdinal":        view.LiveETAOrdinal,
		"liveEtaPlannedArrival": view.LiveETAPlannedArrival,
		"liveEtaLocationId":     view.LiveETALocationID,
		"liveEtaLatitude":       view.LiveETALatitude,
		"liveEtaLongitude":      view.LiveETALongitude,
	}
	return body
}
