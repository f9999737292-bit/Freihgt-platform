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

type BidRevisionHandler struct {
	svc *service.BidRevisionService
}

func NewBidRevisionHandler(svc *service.BidRevisionService) *BidRevisionHandler {
	return &BidRevisionHandler{svc: svc}
}

type submitBidRevisionRequest struct {
	CarrierCompanyID string   `json:"carrier_company_id"`
	TotalAmount      float64  `json:"total_amount"`
	CurrencyCode     string   `json:"currency_code"`
	CapacityUnits    float64  `json:"capacity_units"`
	TransitHours     float64  `json:"transit_hours"`
	SLAScoreInput    float64  `json:"sla_score_input"`
	CarrierKPIInput  float64  `json:"carrier_kpi_score_input"`
	ReliabilityInput float64  `json:"reliability_score_input"`
	Comment          *string  `json:"comment"`
	IdempotencyKey   *string  `json:"idempotency_key"`
}

func (h *BidRevisionHandler) SubmitRevision(w http.ResponseWriter, r *http.Request) {
	bidID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid bid id", map[string]any{"field": "id"}))
		return
	}
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req submitBidRevisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	carrierID, err := uuid.Parse(req.CarrierCompanyID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid carrier_company_id", map[string]any{"field": "carrier_company_id"}))
		return
	}
	carrierScope, _ := resolveOptionalCarrierScope(r)
	if carrierScope != nil && *carrierScope != carrierID {
		respond.Error(w, apperrors.Forbidden("carrier scope mismatch"))
		return
	}
	userID, _ := resolveVerifiedUser(r)
	rev, err := h.svc.SubmitRevision(r.Context(), domain.SubmitBidRevisionInput{
		TenantID:         tenantID,
		BidID:            bidID,
		CarrierCompanyID: carrierID,
		TotalAmount:      req.TotalAmount,
		CurrencyCode:     req.CurrencyCode,
		CapacityUnits:    req.CapacityUnits,
		TransitHours:     req.TransitHours,
		SLAScoreInput:    req.SLAScoreInput,
		CarrierKPIInput:  req.CarrierKPIInput,
		ReliabilityInput: req.ReliabilityInput,
		Comment:          req.Comment,
		IdempotencyKey:   req.IdempotencyKey,
		SubmittedBy:      &userID,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toBidRevisionResponse(rev))
}

func (h *BidRevisionHandler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	bidID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid bid id", map[string]any{"field": "id"}))
		return
	}
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	carrierScope, _ := resolveOptionalCarrierScope(r)
	rev, err := h.svc.GetActiveRevision(r.Context(), bidID, tenantID, carrierScope)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toBidRevisionResponse(rev))
}

func (h *BidRevisionHandler) ListRevisions(w http.ResponseWriter, r *http.Request) {
	bidID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid bid id", map[string]any{"field": "id"}))
		return
	}
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	carrierScope, _ := resolveOptionalCarrierScope(r)
	revisions, err := h.svc.ListRevisions(r.Context(), bidID, tenantID, carrierScope)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(revisions))
	for i := range revisions {
		items = append(items, toBidRevisionResponse(&revisions[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func toBidRevisionResponse(rev *domain.BidRevision) map[string]any {
	return map[string]any{
		"id":                      rev.ID.String(),
		"bid_id":                  rev.BidID.String(),
		"freight_request_id":      rev.FreightRequestID.String(),
		"carrier_company_id":      rev.CarrierCompanyID.String(),
		"revision_number":         rev.RevisionNumber,
		"is_active":               rev.IsActive,
		"total_amount":            rev.TotalAmount,
		"currency_code":           rev.CurrencyCode,
		"capacity_units":          rev.CapacityUnits,
		"transit_hours":           rev.TransitHours,
		"sla_score_input":         rev.SLAScoreInput,
		"carrier_kpi_score_input": rev.CarrierKPIInput,
		"reliability_score_input": rev.ReliabilityInput,
		"comment":                 rev.Comment,
		"submitted_at":            formatDateTime(rev.SubmittedAt),
		"created_at":              rev.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
