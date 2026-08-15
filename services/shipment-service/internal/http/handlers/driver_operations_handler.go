package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
	"github.com/freight-platform/shipment-service/internal/service"
)

type DriverOperationsHandler struct {
	service *service.DriverOperationsService
}

func NewDriverOperationsHandler(svc *service.DriverOperationsService) *DriverOperationsHandler {
	return &DriverOperationsHandler{service: svc}
}

type driverOperationalEventRequest struct {
	Type           string  `json:"type"`
	OccurredAt     *string `json:"occurredAt"`
	IdempotencyKey string  `json:"idempotencyKey"`
}

type driverExceptionRequest struct {
	Category       string  `json:"category"`
	Comment        *string `json:"comment"`
	OccurredAt     *string `json:"occurredAt"`
	IdempotencyKey string  `json:"idempotencyKey"`
	Severity       *string `json:"severity"`
	RuleID         *string `json:"ruleId"`
	PlaybookID     *string `json:"playbookId"`
}

func (h *DriverOperationsHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, err := resolveDriverContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	view, err := h.service.GetMe(r.Context(), tenantID, userID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapDriverMe(view))
}

func (h *DriverOperationsHandler) ListShipments(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, err := resolveDriverContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	filter := domain.ListDriverShipmentsFilter{
		Limit:  parseIntDefault(r.URL.Query().Get("limit"), 50),
		Offset: parseIntDefault(r.URL.Query().Get("offset"), 0),
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
		filter.Status = &status
	}
	items, total, err := h.service.ListShipments(r.Context(), tenantID, userID, filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload = append(payload, mapDriverShipmentSummary(item))
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"items": payload,
		"total": total,
	})
}

func (h *DriverOperationsHandler) GetShipment(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, err := resolveDriverContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipmentID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	detail, err := h.service.GetShipment(r.Context(), tenantID, userID, shipmentID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapDriverShipmentDetail(detail))
}

func (h *DriverOperationsHandler) RecordEvent(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, err := resolveDriverContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipmentID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req driverOperationalEventRequest
	if err := decodeStrictJSON(r, &req); err != nil {
		respond.Error(w, err)
		return
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	}
	input := domain.DriverOperationalEventInput{
		Type:           req.Type,
		IdempotencyKey: idempotencyKey,
	}
	if req.OccurredAt != nil {
		parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(*req.OccurredAt))
		if parseErr != nil {
			respond.Error(w, apperrors.Validation("occurredAt must be RFC3339", map[string]any{"field": "occurredAt"}))
			return
		}
		input.OccurredAt = &parsed
	}
	transition, err := resolveUserStatusTransitionContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.service.RecordOperationalEvent(r.Context(), tenantID, userID, shipmentID, input, transition)
	if err != nil {
		respond.Error(w, err)
		return
	}
	status := http.StatusOK
	if result.Replayed {
		status = http.StatusOK
	}
	respond.JSON(w, status, mapDriverOperationalEventResult(result))
}

func (h *DriverOperationsHandler) ReportException(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, err := resolveDriverContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipmentID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req driverExceptionRequest
	if err := decodeStrictJSON(r, &req); err != nil {
		respond.Error(w, err)
		return
	}
	_ = req.Severity
	_ = req.RuleID
	_ = req.PlaybookID

	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	}
	input := domain.DriverExceptionInput{
		Category:       req.Category,
		Comment:        req.Comment,
		IdempotencyKey: idempotencyKey,
	}
	if req.OccurredAt != nil {
		parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(*req.OccurredAt))
		if parseErr != nil {
			respond.Error(w, apperrors.Validation("occurredAt must be RFC3339", map[string]any{"field": "occurredAt"}))
			return
		}
		input.OccurredAt = &parsed
	}
	var correlationID *string
	if requestID := strings.TrimSpace(sharedmiddleware.RequestIDFromContext(r.Context())); requestID != "" {
		correlationID = &requestID
	}
	result, err := h.service.ReportException(r.Context(), tenantID, userID, shipmentID, input, correlationID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	respond.JSON(w, status, mapDriverExceptionResult(result))
}

func resolveDriverContext(r *http.Request) (tenantID, userID uuid.UUID, err error) {
	tid, err := resolveVerifiedTenant(r)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	uid, err := resolveVerifiedUser(r)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return tid, uid, nil
}

func mapDriverMe(view domain.DriverMeView) map[string]any {
	out := map[string]any{
		"id":              view.ID.String(),
		"displayName":     view.DisplayName,
		"companyId":       view.CompanyID.String(),
		"status":          view.Status,
		"preferredLocale": view.PreferredLocale,
	}
	if view.Phone != nil {
		out["phone"] = *view.Phone
	}
	return out
}

func mapDriverShipmentSummary(item domain.DriverShipmentSummary) map[string]any {
	out := map[string]any{
		"id":                    item.ID.String(),
		"shipmentNumber":        item.ShipmentNumber,
		"status":                item.Status,
		"originLocationId":      item.OriginLocationID.String(),
		"destinationLocationId": item.DestinationLocationID.String(),
	}
	if item.PlannedPickupAt != nil {
		out["plannedPickupAt"] = item.PlannedPickupAt.UTC().Format(time.RFC3339)
	}
	if item.PlannedDeliveryAt != nil {
		out["plannedDeliveryAt"] = item.PlannedDeliveryAt.UTC().Format(time.RFC3339)
	}
	if item.VehicleID != nil {
		out["vehicleId"] = item.VehicleID.String()
	}
	return out
}

func mapDriverShipmentDetail(item domain.DriverShipmentDetail) map[string]any {
	out := mapDriverShipmentSummary(item.DriverShipmentSummary)
	out["transportMode"] = item.TransportMode
	out["version"] = item.Version
	if item.ActualPickupAt != nil {
		out["actualPickupAt"] = item.ActualPickupAt.UTC().Format(time.RFC3339)
	}
	if item.ActualDeliveryAt != nil {
		out["actualDeliveryAt"] = item.ActualDeliveryAt.UTC().Format(time.RFC3339)
	}
	return out
}

func mapDriverOperationalEventResult(result service.DriverOperationalEventResult) map[string]any {
	out := map[string]any{
		"shipmentId":     result.ShipmentID.String(),
		"eventType":      result.EventType,
		"shipmentStatus": result.ShipmentStatus,
		"occurredAt":     result.OccurredAt.UTC().Format(time.RFC3339),
		"receivedAt":     result.ReceivedAt.UTC().Format(time.RFC3339),
		"replayed":       result.Replayed,
	}
	if result.TargetStatus != nil {
		out["targetStatus"] = *result.TargetStatus
	}
	return out
}

func mapDriverExceptionResult(result service.DriverExceptionResult) map[string]any {
	exc := result.Exception
	out := map[string]any{
		"id":             exc.ID.String(),
		"shipmentId":     exc.ShipmentID.String(),
		"driverId":       exc.DriverID.String(),
		"category":       exc.Category,
		"occurredAt":     exc.OccurredAt.UTC().Format(time.RFC3339),
		"receivedAt":     exc.ReceivedAt.UTC().Format(time.RFC3339),
		"source":         exc.Source,
		"idempotencyKey": exc.IdempotencyKey,
		"replayed":       result.Replayed,
	}
	if exc.Comment != nil {
		out["comment"] = *exc.Comment
	}
	if result.OutboxEventID != nil {
		out["outboxEventId"] = result.OutboxEventID.String()
	}
	return out
}

func parseIntDefault(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	var value int
	if _, err := fmt.Sscanf(raw, "%d", &value); err != nil || value <= 0 {
		return fallback
	}
	return value
}
