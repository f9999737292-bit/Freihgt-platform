package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/billing-register-service/internal/domain"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
	"github.com/freight-platform/billing-register-service/internal/platform/respond"
	"github.com/freight-platform/billing-register-service/internal/repository"
	"github.com/freight-platform/billing-register-service/internal/service"
)

type FreightSettlementHandler struct {
	settlements *service.FreightSettlementService
}

func NewFreightSettlementHandler(settlements *service.FreightSettlementService) *FreightSettlementHandler {
	return &FreightSettlementHandler{settlements: settlements}
}

type createSettlementRequest struct {
	ShipmentID       string `json:"shipment_id"`
	IdempotencyKey   string `json:"idempotency_key"`
	SettlementNumber string `json:"settlement_number"`
}

type proposeAccessorialRequest struct {
	ChargeCode         string  `json:"charge_code"`
	Description        *string `json:"description"`
	Amount             float64 `json:"amount"`
	EvidenceDocumentID *string `json:"evidence_document_id"`
	EvidenceType       *string `json:"evidence_type"`
}

type raiseDisputeRequest struct {
	AccessorialID *string `json:"accessorial_id"`
	Reason        string  `json:"reason"`
}

type resolveDisputeRequest struct {
	ResolutionNote string `json:"resolution_note"`
}

type includeRegisterRequest struct {
	RegisterNumber string `json:"register_number"`
}

func (h *FreightSettlementHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, err := settlementActorInput(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req createSettlementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	shipmentID, err := domain.ParseUUID(req.ShipmentID, "shipment_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	settlement, err := h.settlements.Create(r.Context(), domain.CreateFreightSettlementInput{
		TenantID: actor.TenantID, ShipmentID: shipmentID, ActorCompanyID: actor.ActorCompanyID,
		ActorKind: actor.ActorKind, ActorUserID: actor.ActorUserID,
		IdempotencyKey: req.IdempotencyKey, SettlementNumber: req.SettlementNumber,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toSettlementResponse(settlement))
}

func (h *FreightSettlementHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	actor, err := settlementActorInput(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	detail, err := h.settlements.GetDetail(r.Context(), id, actor.TenantID, actor.ActorCompanyID, actor.ActorKind)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toSettlementDetailResponse(detail))
}

func (h *FreightSettlementHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	filter := domain.ListFreightSettlementsFilter{TenantID: tenantID, Limit: parseLimit(r), Offset: parseOffset(r)}
	if raw := strings.TrimSpace(r.URL.Query().Get("buyer_company_id")); raw != "" {
		id, parseErr := domain.ParseUUID(raw, "buyer_company_id")
		if parseErr != nil {
			respond.Error(w, parseErr)
			return
		}
		filter.BuyerCompanyID = &id
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("carrier_company_id")); raw != "" {
		id, parseErr := domain.ParseUUID(raw, "carrier_company_id")
		if parseErr != nil {
			respond.Error(w, parseErr)
			return
		}
		filter.CarrierCompanyID = &id
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		filter.Status = &raw
	}
	items, total, err := h.settlements.List(r.Context(), filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	result := make([]map[string]any, 0, len(items))
	for i := range items {
		result = append(result, toSettlementResponse(&items[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": result, "total": total})
}

func (h *FreightSettlementHandler) ProposeAccessorial(w http.ResponseWriter, r *http.Request) {
	actor, err := settlementActorInput(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	settlementID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req proposeAccessorialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	var evidenceID *uuid.UUID
	if req.EvidenceDocumentID != nil && strings.TrimSpace(*req.EvidenceDocumentID) != "" {
		parsed, parseErr := domain.ParseUUID(*req.EvidenceDocumentID, "evidence_document_id")
		if parseErr != nil {
			respond.Error(w, parseErr)
			return
		}
		evidenceID = &parsed
	}
	item, err := h.settlements.ProposeAccessorial(r.Context(), settlementID, domain.ProposeAccessorialInput{
		SettlementActorInput: actor,
		ChargeCode:           req.ChargeCode,
		Description:          req.Description,
		Amount:               req.Amount,
		EvidenceDocumentID:   evidenceID,
		EvidenceType:         req.EvidenceType,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toAccessorialResponse(item))
}

func (h *FreightSettlementHandler) ApproveAccessorial(w http.ResponseWriter, r *http.Request) {
	h.reviewAccessorial(w, r, true)
}

func (h *FreightSettlementHandler) RejectAccessorial(w http.ResponseWriter, r *http.Request) {
	h.reviewAccessorial(w, r, false)
}

func (h *FreightSettlementHandler) reviewAccessorial(w http.ResponseWriter, r *http.Request, approve bool) {
	actor, err := settlementActorInput(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	settlementID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	accessorialID, err := domain.ParseUUID(chi.URLParam(r, "accessorialId"), "accessorial_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var detail *repository.SettlementDetail
	if approve {
		detail, err = h.settlements.ApproveAccessorial(r.Context(), settlementID, accessorialID, actor)
	} else {
		detail, err = h.settlements.RejectAccessorial(r.Context(), settlementID, accessorialID, actor)
	}
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toSettlementDetailResponse(detail))
}

func (h *FreightSettlementHandler) RaiseDispute(w http.ResponseWriter, r *http.Request) {
	actor, err := settlementActorInput(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	settlementID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req raiseDisputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	var accessorialID *uuid.UUID
	if req.AccessorialID != nil && strings.TrimSpace(*req.AccessorialID) != "" {
		parsed, parseErr := domain.ParseUUID(*req.AccessorialID, "accessorial_id")
		if parseErr != nil {
			respond.Error(w, parseErr)
			return
		}
		accessorialID = &parsed
	}
	dispute, err := h.settlements.RaiseDispute(r.Context(), settlementID, domain.RaiseDisputeInput{
		SettlementActorInput: actor, AccessorialID: accessorialID, Reason: req.Reason,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toDisputeResponse(dispute))
}

func (h *FreightSettlementHandler) ResolveDispute(w http.ResponseWriter, r *http.Request) {
	actor, err := settlementActorInput(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	settlementID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	disputeID, err := domain.ParseUUID(chi.URLParam(r, "disputeId"), "dispute_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req resolveDisputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	detail, err := h.settlements.ResolveDispute(r.Context(), settlementID, disputeID, domain.ResolveDisputeInput{
		SettlementActorInput: actor, ResolutionNote: req.ResolutionNote,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toSettlementDetailResponse(detail))
}

func (h *FreightSettlementHandler) SubmitForReview(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, func(id uuid.UUID, actor domain.SettlementActorInput) (any, error) {
		return h.settlements.SubmitForReview(r.Context(), id, actor)
	})
}

func (h *FreightSettlementHandler) Approve(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, func(id uuid.UUID, actor domain.SettlementActorInput) (any, error) {
		return h.settlements.Approve(r.Context(), id, actor)
	})
}

func (h *FreightSettlementHandler) MarkDocumentsReady(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, func(id uuid.UUID, actor domain.SettlementActorInput) (any, error) {
		return h.settlements.MarkDocumentsReady(r.Context(), id, actor)
	})
}

func (h *FreightSettlementHandler) MarkReadyForPayment(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, func(id uuid.UUID, actor domain.SettlementActorInput) (any, error) {
		return h.settlements.MarkReadyForPayment(r.Context(), id, actor)
	})
}

func (h *FreightSettlementHandler) IncludeInRegister(w http.ResponseWriter, r *http.Request) {
	actor, err := settlementActorInput(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req includeRegisterRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	settlement, err := h.settlements.IncludeInRegister(r.Context(), id, actor, req.RegisterNumber)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toSettlementResponse(settlement))
}

func (h *FreightSettlementHandler) transition(w http.ResponseWriter, r *http.Request, fn func(uuid.UUID, domain.SettlementActorInput) (any, error)) {
	actor, err := settlementActorInput(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := fn(id, actor)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toSettlementResponse(result.(*domain.FreightSettlement)))
}

func toSettlementResponse(s *domain.FreightSettlement) map[string]any {
	return map[string]any{
		"id": s.ID.String(), "tenant_id": s.TenantID.String(),
		"shipment_id": s.ShipmentID.String(), "transport_order_id": s.TransportOrderID.String(),
		"buyer_company_id": s.BuyerCompanyID.String(), "carrier_company_id": s.CarrierCompanyID.String(),
		"award_link_id": optionalUUIDString(s.AwardLinkID), "settlement_number": s.SettlementNumber,
		"base_freight_amount": s.BaseFreightAmount, "currency_code": s.CurrencyCode, "vat_rate": s.VATRate,
		"approved_accessorial_total": s.ApprovedAccessorialTotal,
		"total_without_vat": s.TotalWithoutVAT, "vat_amount": s.VATAmount, "total_with_vat": s.TotalWithVAT,
		"status": s.Status, "service_accepted_at": formatDateTime(s.ServiceAcceptedAt),
		"service_accepted_by": optionalUUIDString(s.ServiceAcceptedBy),
		"billing_register_id": optionalUUIDString(s.BillingRegisterID),
		"billing_register_item_id": optionalUUIDString(s.BillingRegisterItemID),
		"version": s.Version, "created_at": s.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": s.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toSettlementDetailResponse(d *repository.SettlementDetail) map[string]any {
	resp := toSettlementResponse(&d.Settlement)
	accessorials := make([]map[string]any, 0, len(d.Accessorials))
	for i := range d.Accessorials {
		accessorials = append(accessorials, toAccessorialResponse(&d.Accessorials[i]))
	}
	disputes := make([]map[string]any, 0, len(d.Disputes))
	for i := range d.Disputes {
		disputes = append(disputes, toDisputeResponse(&d.Disputes[i]))
	}
	resp["accessorials"] = accessorials
	resp["disputes"] = disputes
	resp["reconciliation"] = map[string]any{
		"base_freight_amount":        d.Settlement.BaseFreightAmount,
		"approved_accessorial_total": d.Settlement.ApprovedAccessorialTotal,
		"settlement_total_without_vat": d.Settlement.TotalWithoutVAT,
		"settlement_total_with_vat":    d.Settlement.TotalWithVAT,
		"currency_code":                d.Settlement.CurrencyCode,
	}
	return resp
}

func toAccessorialResponse(a *domain.SettlementAccessorial) map[string]any {
	return map[string]any{
		"id": a.ID.String(), "settlement_id": a.SettlementID.String(),
		"charge_code": a.ChargeCode, "description": a.Description, "amount": a.Amount,
		"currency_code": a.CurrencyCode, "status": a.Status,
		"submitted_by": a.SubmittedBy.String(), "submitted_by_company_id": a.SubmittedByCompanyID.String(),
		"evidence_document_id": optionalUUIDString(a.EvidenceDocumentID), "evidence_type": a.EvidenceType,
		"created_at": a.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toDisputeResponse(d *domain.SettlementDispute) map[string]any {
	return map[string]any{
		"id": d.ID.String(), "settlement_id": d.SettlementID.String(),
		"accessorial_id": optionalUUIDString(d.AccessorialID), "reason": d.Reason,
		"raised_by": d.RaisedBy.String(), "raised_by_company_id": d.RaisedByCompanyID.String(),
		"status": d.Status, "resolution_note": d.ResolutionNote,
		"resolved_by": optionalUUIDString(d.ResolvedBy), "resolved_at": formatDateTime(d.ResolvedAt),
		"created_at": d.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": d.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
