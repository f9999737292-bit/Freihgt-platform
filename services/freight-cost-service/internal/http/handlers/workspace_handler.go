package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/http/dto"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	"github.com/freight-platform/freight-cost-service/internal/platform/respond"
	"github.com/freight-platform/freight-cost-service/internal/security"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

type WorkspaceHandler struct {
	workspace *service.WorkspaceService
}

func NewWorkspaceHandler(workspace *service.WorkspaceService) *WorkspaceHandler {
	return &WorkspaceHandler{workspace: workspace}
}

func (h *WorkspaceHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, err := security.ParseTrustedActor(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if _, err := ParseTrustedTenant(r); err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.workspace.List(r.Context(), actor, r.URL.Query())
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]dto.WorkspaceSummaryDTO, 0, len(result.Items))
	now := time.Now().UTC()
	for _, item := range result.Items {
		items = append(items, dto.ToWorkspaceSummaryDTO(item, actor, now))
	}
	respond.JSON(w, http.StatusOK, dto.WorkspaceListResponse{
		Items:  items,
		Total:  result.Total,
		Limit:  result.Limit,
		Offset: result.Offset,
	})
}

func (h *WorkspaceHandler) Summary(w http.ResponseWriter, r *http.Request) {
	actor, err := security.ParseTrustedActor(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if _, err := ParseTrustedTenant(r); err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.workspace.Summary(r.Context(), actor, r.URL.Query())
	if err != nil {
		respond.Error(w, err)
		return
	}
	agg := result.Aggregate
	if agg == nil {
		respond.JSON(w, http.StatusOK, dto.ToWorkspaceAggregateResponse("", false, dto.WorkspacePeriodDTO{
			From: result.Period.From, To: result.Period.To, DateDimension: result.Period.DateDimension,
		}, nil, nil, nil, nil, nil, nil, nil, 0))
		return
	}
	respond.JSON(w, http.StatusOK, dto.ToWorkspaceAggregateResponse(
		agg.CurrencyCode,
		agg.MixedCurrency,
		dto.WorkspacePeriodDTO{From: result.Period.From, To: result.Period.To, DateDimension: result.Period.DateDimension},
		agg.PlannedTotal,
		agg.AccruedTotal,
		agg.ForecastExposureTotal,
		agg.CurrentActualTotal,
		agg.FinalActualTotal,
		agg.CurrentVarianceTotal,
		agg.FinalVarianceTotal,
		agg.ReconciliationMismatchCnt,
	))
}

func (h *WorkspaceHandler) Detail(w http.ResponseWriter, r *http.Request) {
	actor, err := security.ParseTrustedActor(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if _, err := ParseTrustedTenant(r); err != nil {
		respond.Error(w, err)
		return
	}
	transportOrderID, err := uuid.Parse(chi.URLParam(r, "transportOrderId"))
	if err != nil || transportOrderID == uuid.Nil {
		respond.Error(w, apperrors.Validation("invalid transport order id", map[string]any{"field": "transport_order_id"}))
		return
	}
	result, err := h.workspace.Detail(r.Context(), actor, transportOrderID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, dto.ToWorkspaceDetailResponse(result, actor))
}

func (h *WorkspaceHandler) VarianceDetail(w http.ResponseWriter, r *http.Request) {
	actor, err := security.ParseTrustedActor(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if _, err := ParseTrustedTenant(r); err != nil {
		respond.Error(w, err)
		return
	}
	transportOrderID, err := uuid.Parse(chi.URLParam(r, "transportOrderId"))
	if err != nil || transportOrderID == uuid.Nil {
		respond.Error(w, apperrors.Validation("invalid transport order id", map[string]any{"field": "transport_order_id"}))
		return
	}
	result, err := h.workspace.VarianceDetail(r.Context(), actor, transportOrderID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, dto.ToWorkspaceVarianceDetailResponse(result))
}

func (h *WorkspaceHandler) AccessorialSummary(w http.ResponseWriter, r *http.Request) {
	actor, err := security.ParseTrustedActor(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if _, err := ParseTrustedTenant(r); err != nil {
		respond.Error(w, err)
		return
	}
	currency, err := h.workspace.AccessorialSummary(r.Context(), actor, r.URL.Query())
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, dto.WorkspaceAccessorialResponse{
		Items:        []dto.WorkspaceAccessorialRowDTO{},
		CurrencyCode: currency,
	})
}

func (h *WorkspaceHandler) CarrierPerformance(w http.ResponseWriter, r *http.Request) {
	actor, err := security.ParseTrustedActor(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if _, err := ParseTrustedTenant(r); err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.workspace.CarrierPerformance(r.Context(), actor, r.URL.Query())
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, dto.ToWorkspaceCarrierPerformanceResponse(result, actor))
}

func (h *WorkspaceHandler) LanePerformance(w http.ResponseWriter, r *http.Request) {
	if _, err := security.ParseTrustedActor(r); err != nil {
		respond.Error(w, err)
		return
	}
	if _, err := ParseTrustedTenant(r); err != nil {
		respond.Error(w, err)
		return
	}
	currency := h.workspace.LanePerformanceCurrency(r.URL.Query())
	respond.JSON(w, http.StatusOK, dto.WorkspaceLanePerformanceResponse{
		Items:        []dto.WorkspaceLanePerformanceRowDTO{},
		CurrencyCode: currency,
	})
}
