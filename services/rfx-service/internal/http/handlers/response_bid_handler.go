package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/service"
)

type ResponseBidHandler struct {
	svc *service.ResponseBidService
}

func NewResponseBidHandler(svc *service.ResponseBidService) *ResponseBidHandler {
	return &ResponseBidHandler{svc: svc}
}

type submitResponseRevisionRequest struct {
	ParticipantCompanyID string   `json:"participant_company_id"`
	PriceAmount          float64  `json:"price_amount"`
	CurrencyCode         string   `json:"currency_code"`
	CapacityUnits        float64  `json:"capacity_units"`
	TransitHours         float64  `json:"transit_hours"`
	SLAScoreInput        float64  `json:"sla_score_input"`
	CarrierKPIInput      float64  `json:"carrier_kpi_score_input"`
	ReliabilityInput     float64  `json:"reliability_score_input"`
	Comment              *string  `json:"comment"`
	IdempotencyKey       *string  `json:"idempotency_key"`
}

func (h *ResponseBidHandler) SubmitRevision(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid event id", map[string]any{"field": "id"}))
		return
	}
	responseID, err := uuid.Parse(chi.URLParam(r, "response_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid response id", map[string]any{"field": "response_id"}))
		return
	}
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req submitResponseRevisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	carrierID, err := uuid.Parse(req.ParticipantCompanyID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid participant_company_id", map[string]any{"field": "participant_company_id"}))
		return
	}
	carrierScope, _ := resolveOptionalCarrierScope(r)
	if carrierScope != nil && *carrierScope != carrierID {
		respond.Error(w, apperrors.Forbidden("carrier scope mismatch"))
		return
	}
	userID, _ := resolveVerifiedUser(r)
	rev, err := h.svc.SubmitRevision(r.Context(), domain.SubmitResponseRevisionInput{
		TenantID:             tenantID,
		RfxEventID:           eventID,
		RfxResponseID:        responseID,
		ParticipantCompanyID: carrierID,
		PriceAmount:          req.PriceAmount,
		CurrencyCode:         req.CurrencyCode,
		CapacityUnits:        req.CapacityUnits,
		TransitHours:         req.TransitHours,
		SLAScoreInput:        req.SLAScoreInput,
		CarrierKPIInput:      req.CarrierKPIInput,
		ReliabilityInput:     req.ReliabilityInput,
		Comment:              req.Comment,
		IdempotencyKey:       req.IdempotencyKey,
		SubmittedBy:          &userID,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toResponseRevisionResponse(rev))
}

func (h *ResponseBidHandler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid event id", map[string]any{"field": "id"}))
		return
	}
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	carrierScope, err := resolveOptionalCarrierScope(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if carrierScope == nil {
		respond.Error(w, apperrors.Validation("carrier scope required", map[string]any{"field": "X-Carrier-Company-ID"}))
		return
	}
	rev, err := h.svc.GetResponseForCarrier(r.Context(), eventID, tenantID, *carrierScope)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toResponseRevisionResponse(rev))
}

func (h *ResponseBidHandler) ListRevisions(w http.ResponseWriter, r *http.Request) {
	responseID, err := uuid.Parse(chi.URLParam(r, "response_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid response id", map[string]any{"field": "response_id"}))
		return
	}
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	carrierScope, _ := resolveOptionalCarrierScope(r)
	revisions, err := h.svc.ListRevisions(r.Context(), responseID, tenantID, carrierScope)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(revisions))
	for i := range revisions {
		items = append(items, toResponseRevisionResponse(&revisions[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ResponseBidHandler) ListEventBids(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid event id", map[string]any{"field": "id"}))
		return
	}
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	carrierScope, _ := resolveOptionalCarrierScope(r)
	bids, err := h.svc.ListEventBids(r.Context(), eventID, tenantID, carrierScope)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(bids))
	for i := range bids {
		items = append(items, toResponseRevisionResponse(&bids[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func toResponseRevisionResponse(rev *domain.RfxResponseRevision) map[string]any {
	return map[string]any{
		"id":                     rev.ID.String(),
		"rfx_response_id":        rev.RfxResponseID.String(),
		"rfx_event_id":           rev.RfxEventID.String(),
		"participant_company_id": rev.ParticipantCompanyID.String(),
		"revision_number":        rev.RevisionNumber,
		"is_active":              rev.IsActive,
		"price_amount":           rev.PriceAmount,
		"currency_code":          rev.CurrencyCode,
		"capacity_units":         rev.CapacityUnits,
		"transit_hours":          rev.TransitHours,
		"sla_score_input":        rev.SLAScoreInput,
		"carrier_kpi_score_input": rev.CarrierKPIInput,
		"reliability_score_input": rev.ReliabilityInput,
		"comment":                rev.Comment,
		"submitted_at":           formatDateTime(rev.SubmittedAt),
		"created_at":             rev.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
