package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/service"
)

type LateSubmissionHandler struct {
	service *service.LateSubmissionService
}

func NewLateSubmissionHandler(svc *service.LateSubmissionService) *LateSubmissionHandler {
	return &LateSubmissionHandler{service: svc}
}

func (h *LateSubmissionHandler) CreateRequest(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	carrierCompanyID, err := parseCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req struct {
		ReasonCode     string `json:"reason_code"`
		ReasonText     string `json:"reason_text"`
		RequestedUntil string `json:"requested_until"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	reasonCode, err := domain.ParseLateSubmissionReasonCode(req.ReasonCode)
	if err != nil {
		respond.Error(w, err)
		return
	}
	requestedUntil, err := time.Parse(time.RFC3339, req.RequestedUntil)
	if err != nil {
		respond.Error(w, apperrors.Validation("requested_until must be RFC3339", map[string]any{"field": "requested_until"}))
		return
	}
	out, err := h.service.CreateRequest(r.Context(), actor, eventID, carrierCompanyID, r.Header.Get("Idempotency-Key"), domain.CreateLateSubmissionRequestInput{
		ReasonCode: reasonCode, ReasonText: req.ReasonText, RequestedUntil: requestedUntil.UTC(),
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toLateSubmissionRequestResponse(out))
}

func (h *LateSubmissionHandler) ListOwnRequests(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	carrierCompanyID, err := parseCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items, err := h.service.ListOwnRequests(r.Context(), actor, eventID, carrierCompanyID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": toLateSubmissionRequestResponses(items)})
}

func (h *LateSubmissionHandler) BuyerListRequests(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	items, err := h.service.BuyerListRequests(r.Context(), actor, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": toLateSubmissionRequestResponses(items)})
}

func (h *LateSubmissionHandler) Approve(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	requestID, ok := parseLateSubmissionRequestID(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedVersion    int    `json:"expected_version"`
		ApprovedValidFrom  string `json:"approved_valid_from"`
		ApprovedValidUntil string `json:"approved_valid_until"`
		DecisionComment    string `json:"decision_comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	validFrom, err := time.Parse(time.RFC3339, req.ApprovedValidFrom)
	if err != nil {
		respond.Error(w, apperrors.Validation("approved_valid_from must be RFC3339", map[string]any{"field": "approved_valid_from"}))
		return
	}
	validUntil, err := time.Parse(time.RFC3339, req.ApprovedValidUntil)
	if err != nil {
		respond.Error(w, apperrors.Validation("approved_valid_until must be RFC3339", map[string]any{"field": "approved_valid_until"}))
		return
	}
	out, err := h.service.Approve(r.Context(), actor, eventID, requestID, r.Header.Get("Idempotency-Key"), domain.ApproveLateSubmissionInput{
		ExpectedVersion: req.ExpectedVersion, ApprovedValidFrom: validFrom.UTC(),
		ApprovedValidUntil: validUntil.UTC(), DecisionComment: req.DecisionComment,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toLateSubmissionRequestResponse(out))
}

func (h *LateSubmissionHandler) Reject(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	requestID, ok := parseLateSubmissionRequestID(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedVersion int    `json:"expected_version"`
		DecisionComment string `json:"decision_comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	out, err := h.service.Reject(r.Context(), actor, eventID, requestID, r.Header.Get("Idempotency-Key"), domain.RejectLateSubmissionInput{
		ExpectedVersion: req.ExpectedVersion, DecisionComment: req.DecisionComment,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toLateSubmissionRequestResponse(out))
}

func parseLateSubmissionRequestID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := domain.ParseUUID(chi.URLParam(r, "request_id"), "request_id")
	if err != nil {
		respond.Error(w, err)
		return uuid.Nil, false
	}
	return id, true
}

func toLateSubmissionRequestResponses(items []domain.LateSubmissionRequest) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		out = append(out, toLateSubmissionRequestResponse(&items[i]))
	}
	return out
}

func toLateSubmissionRequestResponse(req *domain.LateSubmissionRequest) map[string]any {
	now := time.Now().UTC()
	status := domain.EffectiveLateSubmissionStatus(req, now)
	resp := map[string]any{
		"id":                 req.ID.String(),
		"rfx_event_id":       req.RfxEventID.String(),
		"carrier_company_id": req.CarrierCompanyID.String(),
		"reason_code":        string(req.ReasonCode),
		"reason_text":        req.ReasonText,
		"requested_until":    req.RequestedUntil.UTC().Format(time.RFC3339),
		"status":             string(status),
		"requested_by":       req.RequestedBy.String(),
		"version":            req.Version,
		"created_at":         req.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":         req.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if req.ParticipantID != nil {
		resp["participant_id"] = req.ParticipantID.String()
	}
	if req.ApprovedValidFrom != nil {
		resp["approved_valid_from"] = req.ApprovedValidFrom.UTC().Format(time.RFC3339)
	}
	if req.ApprovedValidUntil != nil {
		resp["approved_valid_until"] = req.ApprovedValidUntil.UTC().Format(time.RFC3339)
	}
	if req.DecisionComment != nil {
		resp["decision_comment"] = *req.DecisionComment
	}
	if req.DecidedBy != nil {
		resp["decided_by"] = req.DecidedBy.String()
	}
	if req.DecidedAt != nil {
		resp["decided_at"] = req.DecidedAt.UTC().Format(time.RFC3339)
	}
	if req.ConsumedAt != nil {
		resp["consumed_at"] = req.ConsumedAt.UTC().Format(time.RFC3339)
	}
	return resp
}
