package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
)

const headerInternalServiceName = "X-Internal-Service-Name"

type executionProjector interface {
	CreateExecutionProjectionFromActivation(ctx context.Context, cmd domain.ProjectionCommand) (domain.ProjectionResult, error)
}

type TransportExecutionHandler struct {
	projector executionProjector
}

func NewTransportExecutionHandler(projector executionProjector) *TransportExecutionHandler {
	return &TransportExecutionHandler{projector: projector}
}

func (h *TransportExecutionHandler) CreateFromActivation(w http.ResponseWriter, r *http.Request) {
	caller := strings.TrimSpace(r.Header.Get(headerInternalServiceName))
	if caller != domain.TrustedProjectionCaller {
		respond.Error(w, apperrors.Forbidden("projection caller is not network-optimizer-service"))
		return
	}
	body := http.MaxBytesReader(w, r.Body, 1<<20)
	defer body.Close()
	var cmd domain.ProjectionCommand
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cmd); err != nil {
		respond.Error(w, apperrors.Validation("invalid projection body", map[string]any{"field": "body"}))
		return
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF {
		respond.Error(w, apperrors.Validation("invalid projection body", map[string]any{"field": "body"}))
		return
	}
	result, err := h.projector.CreateExecutionProjectionFromActivation(r.Context(), cmd)
	if err != nil {
		respond.Error(w, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	respond.JSON(w, status, map[string]string{
		"execution_id":  result.ExecutionID.String(),
		"revision_id":   result.RevisionID.String(),
		"activation_id": result.ActivationID.String(),
	})
}
