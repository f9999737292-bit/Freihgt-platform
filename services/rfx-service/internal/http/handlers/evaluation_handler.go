package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type updateResponseCommercialRequest struct {
	OfferLines []offerLineRequest `json:"offer_lines"`
}

type offerLineRequest struct {
	RfxLotID     *string `json:"rfx_lot_id"`
	Amount       float64 `json:"amount"`
	CurrencyCode string  `json:"currency_code"`
	Comment      *string `json:"comment"`
}

type updateManualScoreRequest struct {
	ManualScore float64 `json:"manual_score"`
}

type awardResponseRequest struct {
	ResponseID string `json:"response_id"`
}

func (h *RfxHandler) UpdateResponseCommercial(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	responseID, err := domain.ParseUUID(chi.URLParam(r, "response_id"), "response_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req updateResponseCommercialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	lines, err := parseOfferLines(req.OfferLines)
	if err != nil {
		respond.Error(w, err)
		return
	}
	response, err := h.service.UpdateResponseCommercial(r.Context(), actor, responseID, lines)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRfxResponseDetailResponse(response, nil))
}

func (h *RfxHandler) ListEvaluationResponses(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	views, err := h.service.ListEvaluationResponses(r.Context(), actor, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(views))
	for i := range views {
		items = append(items, toEvaluationResponseView(&views[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *RfxHandler) RecalculateEvaluation(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	views, err := h.service.RecalculateEvaluation(r.Context(), actor, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(views))
	for i := range views {
		items = append(items, toEvaluationResponseView(&views[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *RfxHandler) UpdateManualScore(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	responseID, err := domain.ParseUUID(chi.URLParam(r, "response_id"), "response_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req updateManualScoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	response, err := h.service.UpdateManualScore(r.Context(), actor, responseID, req.ManualScore)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRfxResponseDetailResponse(response, nil))
}

func (h *RfxHandler) AddResponseToShortlist(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	responseID, err := domain.ParseUUID(chi.URLParam(r, "response_id"), "response_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	if err := h.service.AddToShortlist(r.Context(), actor, responseID); err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"status": "SHORTLISTED"})
}

func (h *RfxHandler) RemoveResponseFromShortlist(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	responseID, err := domain.ParseUUID(chi.URLParam(r, "response_id"), "response_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	if err := h.service.RemoveFromShortlist(r.Context(), actor, responseID); err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"status": "RESPONSE_SUBMITTED"})
}

func (h *RfxHandler) AwardResponse(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req awardResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	responseID, err := domain.ParseUUID(req.ResponseID, "response_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	award, err := h.service.AwardResponse(r.Context(), actor, eventID, responseID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRfxAwardResponse(award))
}

func (h *RfxHandler) ListEventAuditEvents(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	events, err := h.service.ListEventAuditEvents(r.Context(), actor, eventID, 100)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(events))
	for _, event := range events {
		items = append(items, toAuditEventResponse(event))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *RfxHandler) GetOwnAward(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	carrierCompanyID, err := parseOptionalCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	award, err := h.service.GetOwnAward(r.Context(), actor, eventID, carrierCompanyID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRfxAwardResponse(award))
}

func parseOfferLines(items []offerLineRequest) ([]domain.UpsertOfferLineInput, error) {
	lines := make([]domain.UpsertOfferLineInput, 0, len(items))
	for _, item := range items {
		line := domain.UpsertOfferLineInput{
			Amount:       item.Amount,
			CurrencyCode: item.CurrencyCode,
			Comment:      item.Comment,
		}
		if item.RfxLotID != nil && *item.RfxLotID != "" {
			lotID, err := domain.ParseUUID(*item.RfxLotID, "rfx_lot_id")
			if err != nil {
				return nil, err
			}
			line.RfxLotID = lotID
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func toEvaluationResponseView(view *domain.EvaluationResponseView) map[string]any {
	resp := toRfxResponseDetailResponse(&view.Response, view)
	resp["participant_status"] = view.ParticipantStatus
	resp["total_amount"] = view.TotalAmount
	resp["currency_code"] = view.CurrencyCode
	resp["comparable"] = view.Comparable
	resp["shortlisted"] = view.Shortlisted
	resp["awarded"] = view.Awarded
	resp["offer_complete"] = view.OfferComplete
	if view.Response.CommercialScore != nil {
		resp["commercial_score"] = *view.Response.CommercialScore
	}
	if view.Response.ManualScore != nil {
		resp["manual_score"] = *view.Response.ManualScore
	}
	if view.Response.TotalScore != nil {
		resp["total_score"] = *view.Response.TotalScore
	}
	if view.Response.EvaluationRank != nil {
		resp["rank"] = *view.Response.EvaluationRank
	}
	return resp
}

func toRfxResponseDetailResponse(response *domain.RfxResponse, view *domain.EvaluationResponseView) map[string]any {
	resp := toRfxResponseResponse(response)
	lines := make([]map[string]any, 0, len(response.OfferLines))
	for i := range response.OfferLines {
		line := response.OfferLines[i]
		item := map[string]any{
			"id":            line.ID.String(),
			"amount":        line.Amount,
			"currency_code": line.CurrencyCode,
			"comment":       line.Comment,
		}
		if line.RfxLotID != uuid.Nil {
			item["rfx_lot_id"] = line.RfxLotID.String()
		}
		lines = append(lines, item)
	}
	resp["offer_lines"] = lines
	if view != nil {
		resp["total_amount"] = view.TotalAmount
		resp["currency_code"] = view.CurrencyCode
		resp["comparable"] = view.Comparable
	}
	return resp
}

func toRfxAwardResponse(award *domain.RfxAward) map[string]any {
	resp := map[string]any{
		"id":                 award.ID.String(),
		"tenant_id":          award.TenantID.String(),
		"rfx_event_id":       award.RfxEventID.String(),
		"rfx_response_id":    award.RfxResponseID.String(),
		"carrier_company_id": award.CarrierCompanyID.String(),
		"awarded_at":         award.AwardedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if award.TotalAmount != nil {
		resp["total_amount"] = *award.TotalAmount
	}
	if award.CurrencyCode != nil {
		resp["currency_code"] = *award.CurrencyCode
	}
	if award.AwardedBy != nil {
		resp["awarded_by"] = award.AwardedBy.String()
	}
	return resp
}

func toAuditEventResponse(event repository.AuditEvent) map[string]any {
	resp := map[string]any{
		"id":          event.ID.String(),
		"entity_type": event.EntityType,
		"entity_id":   event.EntityID.String(),
		"action":      event.Action,
		"metadata":    event.Metadata,
		"created_at":  event.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if event.ActorUserID != nil {
		resp["actor_user_id"] = event.ActorUserID.String()
	}
	if event.ActorCompanyID != nil {
		resp["actor_company_id"] = event.ActorCompanyID.String()
	}
	return resp
}
