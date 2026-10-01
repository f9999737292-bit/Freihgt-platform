package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
)

// DeliveryDispositionWriter records rejection and disposition decisions.
// Operating tenant comes from the verified header, never from the body.
type DeliveryDispositionWriter interface {
	RecordDeliveryDisposition(ctx context.Context, cmd domain.RecordDeliveryDispositionCommand) (domain.DispositionResult, error)
	AuthorizeReturn(ctx context.Context, cmd domain.AuthorizeDispositionCommand) (domain.DispositionResult, error)
	AuthorizeRedirect(ctx context.Context, cmd domain.AuthorizeDispositionCommand) (domain.DispositionResult, error)
	HoldDisposition(ctx context.Context, cmd domain.HoldDispositionCommand) (domain.DispositionResult, error)
	CompleteDisposition(ctx context.Context, cmd domain.CompleteDispositionCommand) (domain.DispositionResult, error)
	ListDeliveryDispositions(ctx context.Context, operatingTenant, executionID uuid.UUID) ([]domain.DispositionView, error)
}

type DeliveryDispositionHandler struct {
	writer DeliveryDispositionWriter
}

func NewDeliveryDispositionHandler(writer DeliveryDispositionWriter) *DeliveryDispositionHandler {
	return &DeliveryDispositionHandler{writer: writer}
}

type dispositionEvidenceBody struct {
	EvidenceType string `json:"evidenceType"`
	Source       string `json:"source"`
	ReferenceID  string `json:"referenceId"`
}

type recordDispositionBody struct {
	ExpectedCurrentRevisionID string                    `json:"expectedCurrentRevisionId"`
	SourceExecutionStopID     string                    `json:"sourceExecutionStopId"`
	ShipmentID                string                    `json:"shipmentId"`
	CargoID                   string                    `json:"cargoId"`
	AcceptedQuantity          int                       `json:"acceptedQuantity"`
	RejectedQuantity          int                       `json:"rejectedQuantity"`
	UOM                       string                    `json:"uom"`
	ReasonCode                string                    `json:"reasonCode"`
	ReasonComment             string                    `json:"reasonComment"`
	IdempotencyKey            string                    `json:"idempotencyKey"`
	OccurredAt                string                    `json:"occurredAt"`
	ActorKind                 string                    `json:"actorKind"`
	ActorID                   string                    `json:"actorId"`
	Evidence                  []dispositionEvidenceBody `json:"evidence"`
}

type authorizeDispositionBody struct {
	ExecutionID               string   `json:"executionId"`
	ExpectedCurrentRevisionID string   `json:"expectedCurrentRevisionId"`
	TargetLocationID          string   `json:"targetLocationId"`
	FutureStopIDs             []string `json:"futureStopIds"`
	InsertAt                  *int     `json:"insertAt"`
	Instruction               string   `json:"instruction"`
	IdempotencyKey            string   `json:"idempotencyKey"`
	OccurredAt                string   `json:"occurredAt"`
	ActorKind                 string   `json:"actorKind"`
	ActorID                   string   `json:"actorId"`
}

type holdDispositionBody struct {
	ExpectedCurrentRevisionID string `json:"expectedCurrentRevisionId"`
	ExecutionID               string `json:"executionId"`
	Instruction               string `json:"instruction"`
	IdempotencyKey            string `json:"idempotencyKey"`
	OccurredAt                string `json:"occurredAt"`
	ActorKind                 string `json:"actorKind"`
	ActorID                   string `json:"actorId"`
}

type completeDispositionBody struct {
	ExpectedCurrentRevisionID string `json:"expectedCurrentRevisionId"`
	ExecutionID               string `json:"executionId"`
	ActionID                  string `json:"actionId"`
	IdempotencyKey            string `json:"idempotencyKey"`
	OccurredAt                string `json:"occurredAt"`
	ActorKind                 string `json:"actorKind"`
	ActorID                   string `json:"actorId"`
}

func (h *DeliveryDispositionHandler) Record(w http.ResponseWriter, r *http.Request) {
	tenantID, executionID, ok := dispositionIdentity(w, r)
	if !ok {
		return
	}
	var body recordDispositionBody
	if !decodeDisposition(w, r, &body) {
		return
	}
	stopID, err := domain.ParseUUID(body.SourceExecutionStopID, "source_execution_stop_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipmentID, err := domain.ParseUUID(body.ShipmentID, "shipment_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	cargoID, err := domain.ParseUUID(body.CargoID, "cargo_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	expected, occurred, actorID, err := dispositionMeta(body.ExpectedCurrentRevisionID, body.OccurredAt, body.ActorID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	evidence := make([]domain.DeliveryEvidenceRef, len(body.Evidence))
	for i, item := range body.Evidence {
		evidence[i] = domain.DeliveryEvidenceRef{EvidenceType: item.EvidenceType, Source: item.Source, ReferenceID: item.ReferenceID}
	}
	result, err := h.writer.RecordDeliveryDisposition(r.Context(), domain.RecordDeliveryDispositionCommand{
		ExecutionID: executionID, ExpectedRevisionID: expected, OperatingTenantID: tenantID,
		SourceStopID: stopID, ShipmentID: shipmentID, CargoID: cargoID,
		AcceptedQuantity: body.AcceptedQuantity, RejectedQuantity: body.RejectedQuantity, UOM: body.UOM,
		ReasonCode: body.ReasonCode, ReasonComment: body.ReasonComment, IdempotencyKey: body.IdempotencyKey,
		OccurredAt: occurred, ActorKind: body.ActorKind, ActorID: actorID, Evidence: evidence,
	})
	writeDisposition(w, result, err)
}

func (h *DeliveryDispositionHandler) AuthorizeReturn(w http.ResponseWriter, r *http.Request) {
	h.authorize(w, r, true)
}

func (h *DeliveryDispositionHandler) AuthorizeRedirect(w http.ResponseWriter, r *http.Request) {
	h.authorize(w, r, false)
}

func (h *DeliveryDispositionHandler) authorize(w http.ResponseWriter, r *http.Request, returnToOrigin bool) {
	tenantID, caseID, ok := caseIdentity(w, r)
	if !ok {
		return
	}
	var body authorizeDispositionBody
	if !decodeDisposition(w, r, &body) {
		return
	}
	expected, occurred, actorID, err := dispositionMeta(body.ExpectedCurrentRevisionID, body.OccurredAt, body.ActorID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	target, err := domain.ParseUUID(body.TargetLocationID, "target_location_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	executionID, err := domain.ParseUUID(body.ExecutionID, "execution_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	stops := make([]uuid.UUID, len(body.FutureStopIDs))
	for i, raw := range body.FutureStopIDs {
		id, parseErr := domain.ParseUUID(raw, "future_stop_id")
		if parseErr != nil {
			respond.Error(w, parseErr)
			return
		}
		stops[i] = id
	}
	cmd := domain.AuthorizeDispositionCommand{
		CaseID: caseID, ExecutionID: executionID, ExpectedRevisionID: expected, OperatingTenantID: tenantID, TargetLocationID: target,
		FutureStopIDs: stops, InsertAt: body.InsertAt, Instruction: body.Instruction,
		IdempotencyKey: body.IdempotencyKey, OccurredAt: occurred, ActorKind: body.ActorKind, ActorID: actorID,
	}
	var result domain.DispositionResult
	if returnToOrigin {
		result, err = h.writer.AuthorizeReturn(r.Context(), cmd)
	} else {
		result, err = h.writer.AuthorizeRedirect(r.Context(), cmd)
	}
	writeDisposition(w, result, err)
}

func (h *DeliveryDispositionHandler) Hold(w http.ResponseWriter, r *http.Request) {
	tenantID, caseID, ok := caseIdentity(w, r)
	if !ok {
		return
	}
	var body holdDispositionBody
	if !decodeDisposition(w, r, &body) {
		return
	}
	expected, occurred, actorID, err := dispositionMeta(body.ExpectedCurrentRevisionID, body.OccurredAt, body.ActorID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	executionID, err := domain.ParseUUID(body.ExecutionID, "execution_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.writer.HoldDisposition(r.Context(), domain.HoldDispositionCommand{
		CaseID: caseID, ExecutionID: executionID, ExpectedRevisionID: expected, OperatingTenantID: tenantID,
		Instruction: body.Instruction, IdempotencyKey: body.IdempotencyKey, OccurredAt: occurred,
		ActorKind: body.ActorKind, ActorID: actorID,
	})
	writeDisposition(w, result, err)
}

func (h *DeliveryDispositionHandler) Complete(w http.ResponseWriter, r *http.Request) {
	tenantID, caseID, ok := caseIdentity(w, r)
	if !ok {
		return
	}
	var body completeDispositionBody
	if !decodeDisposition(w, r, &body) {
		return
	}
	expected, occurred, actorID, err := dispositionMeta(body.ExpectedCurrentRevisionID, body.OccurredAt, body.ActorID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	executionID, err := domain.ParseUUID(body.ExecutionID, "execution_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	actionID, err := domain.ParseUUID(body.ActionID, "action_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.writer.CompleteDisposition(r.Context(), domain.CompleteDispositionCommand{
		CaseID: caseID, ExecutionID: executionID, ExpectedRevisionID: expected, OperatingTenantID: tenantID,
		ActionID: actionID, IdempotencyKey: body.IdempotencyKey, OccurredAt: occurred,
		ActorKind: body.ActorKind, ActorID: actorID,
	})
	writeDisposition(w, result, err)
}

func (h *DeliveryDispositionHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, executionID, ok := dispositionIdentity(w, r)
	if !ok {
		return
	}
	items, err := h.writer.ListDeliveryDispositions(r.Context(), tenantID, executionID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if items == nil {
		items = []domain.DispositionView{}
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func dispositionIdentity(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return uuid.Nil, uuid.Nil, false
	}
	executionID, err := domain.ParseUUID(chi.URLParam(r, "executionId"), "execution_id")
	if err != nil {
		respond.Error(w, err)
		return uuid.Nil, uuid.Nil, false
	}
	return tenantID, executionID, true
}

func caseIdentity(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return uuid.Nil, uuid.Nil, false
	}
	caseID, err := domain.ParseUUID(chi.URLParam(r, "caseId"), "case_id")
	if err != nil {
		respond.Error(w, err)
		return uuid.Nil, uuid.Nil, false
	}
	return tenantID, caseID, true
}

func decodeDisposition(w http.ResponseWriter, r *http.Request, dest any) bool {
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		respond.Error(w, apperrors.Validation("request body is invalid", map[string]any{"field": "body"}))
		return false
	}
	return true
}

func dispositionMeta(expectedRaw, occurredRaw, actorRaw string) (uuid.UUID, time.Time, uuid.UUID, error) {
	expected, err := domain.ParseUUID(expectedRaw, "expected_current_revision_id")
	if err != nil {
		return uuid.Nil, time.Time{}, uuid.Nil, err
	}
	occurred, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(occurredRaw))
	if err != nil {
		occurred, err = time.Parse(time.RFC3339, strings.TrimSpace(occurredRaw))
		if err != nil {
			return uuid.Nil, time.Time{}, uuid.Nil, apperrors.Validation("occurred_at is invalid", map[string]any{"field": "occurred_at"})
		}
	}
	actorID := uuid.Nil
	if strings.TrimSpace(actorRaw) != "" {
		actorID, err = domain.ParseUUID(actorRaw, "actor_id")
		if err != nil {
			return uuid.Nil, time.Time{}, uuid.Nil, err
		}
	}
	return expected, occurred, actorID, nil
}

func writeDisposition(w http.ResponseWriter, result domain.DispositionResult, err error) {
	if err != nil {
		respond.Error(w, err)
		return
	}
	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	body := map[string]any{
		"commandId": result.CommandID, "executionId": result.ExecutionID, "status": result.Status,
		"acceptedQuantity": result.AcceptedQuantity, "rejectedQuantity": result.RejectedQuantity,
		"revisionId": result.RevisionID, "replayed": result.Replayed,
	}
	if result.CaseID != nil {
		body["caseId"] = *result.CaseID
	}
	respond.JSON(w, status, body)
}
