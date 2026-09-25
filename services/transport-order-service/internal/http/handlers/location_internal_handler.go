package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
	"github.com/freight-platform/transport-order-service/internal/platform/respond"
)

type locationReader interface {
	GetLocation(ctx context.Context, tenantID, id uuid.UUID) (*domain.Location, error)
}

type LocationInternalHandler struct {
	locations locationReader
}

func NewLocationInternalHandler(locations locationReader) *LocationInternalHandler {
	return &LocationInternalHandler{locations: locations}
}

func (h *LocationInternalHandler) GetProjection(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "locationId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid location id", map[string]any{"field": "location_id"}))
		return
	}
	location, err := h.locations.GetLocation(r.Context(), tenantID, id)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if location == nil || location.TenantID != tenantID || location.Status != "ACTIVE" {
		respond.Error(w, apperrors.NotFound("location not found"))
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"id":           location.ID,
		"country_code": location.CountryCode,
		"region":       location.Region,
		"city":         location.City,
		"lat":          location.Lat,
		"lon":          location.Lon,
		"timezone":     location.Timezone,
		"status":       location.Status,
		"version":      location.Version,
	})
}
