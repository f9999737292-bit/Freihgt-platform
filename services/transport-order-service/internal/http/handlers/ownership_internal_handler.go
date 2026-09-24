package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
	"github.com/freight-platform/transport-order-service/internal/platform/respond"
)

type ownershipConfirmer interface {
	ConfirmOwnership(ctx context.Context, tenantID, id uuid.UUID) error
}

type OwnershipInternalHandler struct {
	confirm ownershipConfirmer
}

func NewOwnershipInternalHandler(confirm ownershipConfirmer) *OwnershipInternalHandler {
	return &OwnershipInternalHandler{confirm: confirm}
}

func (h *OwnershipInternalHandler) GetTransportOrder(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "transportOrderId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid transport order id", map[string]any{"field": "transport_order_id"}))
		return
	}
	if err := h.confirm.ConfirmOwnership(r.Context(), tenantID, id); err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{
		"id":        id.String(),
		"tenant_id": tenantID.String(),
	})
}
