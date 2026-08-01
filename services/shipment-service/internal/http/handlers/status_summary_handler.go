package handlers

import (
	"net/http"
	"strings"
	"time"

	shipmentmetrics "github.com/freight-platform/shipment-service/internal/platform/metrics"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
	"github.com/freight-platform/shipment-service/internal/service"
)

type StatusSummaryHandler struct {
	service *service.StatusSummaryService
	metrics *shipmentmetrics.StatusSummaryMetrics
}

func NewStatusSummaryHandler(svc *service.StatusSummaryService) *StatusSummaryHandler {
	return &StatusSummaryHandler{
		service: svc,
		metrics: shipmentmetrics.NewStatusSummaryMetrics(),
	}
}

func (h *StatusSummaryHandler) GetStatusSummary(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	result := "success"
	errorCode := "NONE"
	defer func() {
		h.metrics.Observe(result, errorCode, time.Since(started))
	}()

	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		result = "error"
		errorCode = classifyStatusSummaryError(err)
		respond.Error(w, err)
		return
	}

	summary, err := h.service.GetStatusSummary(r.Context(), tenantID)
	if err != nil {
		result = "error"
		errorCode = classifyStatusSummaryError(err)
		respond.Error(w, err)
		return
	}

	response := map[string]any{
		"totalShipments":   summary.TotalShipments,
		"countedShipments": summary.CountedShipments,
		"byStatus":         summary.ByStatus,
		"complete":         summary.Complete,
		"calculatedAt":     summary.CalculatedAt.UTC().Format(time.RFC3339),
	}
	if len(summary.Warnings) > 0 {
		response["warnings"] = summary.Warnings
	}
	respond.JSON(w, http.StatusOK, response)
}

func classifyStatusSummaryError(err error) string {
	if err == nil {
		return "NONE"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "validation"), strings.Contains(msg, "tenant_id"):
		return "VALIDATION"
	case strings.Contains(msg, "unauthorized"), strings.Contains(msg, "tenant context"):
		return "UNAUTHORIZED"
	default:
		return "INTERNAL"
	}
}
