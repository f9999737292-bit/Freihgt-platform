package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
)

type predictionInputReader interface {
	PredictionInput(ctx context.Context, tenantID, id uuid.UUID) (*domain.ShipmentPredictionInput, error)
}

type PredictionInputHandler struct {
	reader predictionInputReader
}

func NewPredictionInputHandler(reader predictionInputReader) *PredictionInputHandler {
	return &PredictionInputHandler{reader: reader}
}

func (h *PredictionInputHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	id, err := domain.ParseUUID(chi.URLParam(r, "shipmentId"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	input, err := h.reader.PredictionInput(r.Context(), tenantID, id)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, input)
}
