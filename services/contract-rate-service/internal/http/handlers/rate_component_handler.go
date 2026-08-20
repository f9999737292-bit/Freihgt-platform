package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	"github.com/freight-platform/contract-rate-service/internal/platform/respond"
	"github.com/freight-platform/contract-rate-service/internal/service"
)

type RateComponentHandler struct {
	svc    *service.RateComponentService
	actors *ActorResolver
}

func NewRateComponentHandler(svc *service.RateComponentService, actors *ActorResolver) *RateComponentHandler {
	return &RateComponentHandler{svc: svc, actors: actors}
}

func (h *RateComponentHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	lineID, err := parsePathUUID(chi.URLParam(r, "lineId"), "line_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req struct {
		ComponentType     string  `json:"component_type"`
		CalculationMethod string  `json:"calculation_method"`
		Amount            *string `json:"amount"`
		PercentValue      *string `json:"percent_value"`
		UnitCode          *string `json:"unit_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, domainValidation("invalid request body"))
		return
	}
	amount, err := parseOptionalDecimal(req.Amount, "amount")
	if err != nil {
		respond.Error(w, err)
		return
	}
	percent, err := parseOptionalDecimal(req.PercentValue, "percent_value")
	if err != nil {
		respond.Error(w, err)
		return
	}
	created, err := h.svc.Create(r.Context(), domain.CreateRateComponentInput{
		TenantID: actor.TenantID, RateLineID: lineID,
		ComponentType: req.ComponentType, CalculationMethod: req.CalculationMethod,
		Amount: amount, PercentValue: percent, UnitCode: req.UnitCode, Actor: actor,
	}, CorrelationID(r))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, mapRateComponent(created))
}

func (h *RateComponentHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	lineID, err := parsePathUUID(chi.URLParam(r, "lineId"), "line_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	items, err := h.svc.ListByLine(r.Context(), actor.TenantID, lineID, actor)
	if err != nil {
		respond.Error(w, err)
		return
	}
	out := make([]any, 0, len(items))
	for i := range items {
		out = append(out, mapRateComponent(&items[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *RateComponentHandler) Patch(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	componentID, err := parsePathUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req struct {
		Amount       *string `json:"amount"`
		PercentValue *string `json:"percent_value"`
		UnitCode     *string `json:"unit_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, domainValidation("invalid request body"))
		return
	}
	amount, err := parseOptionalDecimal(req.Amount, "amount")
	if err != nil {
		respond.Error(w, err)
		return
	}
	percent, err := parseOptionalDecimal(req.PercentValue, "percent_value")
	if err != nil {
		respond.Error(w, err)
		return
	}
	updated, err := h.svc.Update(r.Context(), actor.TenantID, componentID, domain.UpdateRateComponentInput{
		Amount: amount, PercentValue: percent, UnitCode: req.UnitCode, Actor: actor,
	}, CorrelationID(r))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapRateComponent(updated))
}

func (h *RateComponentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	componentID, err := parsePathUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	if err := h.svc.Delete(r.Context(), actor.TenantID, componentID, actor, CorrelationID(r)); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func mapRateComponent(c *domain.RateComponent) map[string]any {
	return map[string]any{
		"id": c.ID, "tenant_id": c.TenantID, "rate_line_id": c.RateLineID,
		"component_type": c.ComponentType, "calculation_method": c.CalculationMethod,
		"amount": decimalStringPtr(c.Amount), "percent_value": decimalStringPtr(c.PercentValue),
		"unit_code": c.UnitCode, "created_at": c.CreatedAt, "updated_at": c.UpdatedAt,
	}
}
