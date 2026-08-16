package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	DriverEventErrorInvalidJSON      = "INVALID_JSON"
	DriverEventErrorMissingTenant    = "MISSING_TENANT"
	DriverEventErrorMissingShipment  = "MISSING_SHIPMENT"
	DriverEventErrorMissingEventID   = "MISSING_EVENT_ID"
	DriverEventErrorUnknownEventType = "UNKNOWN_EVENT_TYPE"
	DriverEventErrorUnsupportedVersion = "UNSUPPORTED_SCHEMA_VERSION"
	DriverEventErrorTenantMismatch   = "TENANT_MISMATCH"
)

var allowedIngestDriverEventTypes = map[string]struct{}{
	"driver.location.updated":        {},
	"driver.arrived_at_pickup":       {},
	"driver.departed_pickup":         {},
	"driver.arrived_at_delivery":     {},
	"driver.delivery.completed":      {},
	"driver.delay.reported":          {},
	"driver.problem.reported":        {},
	"driver.documents.uploaded":      {},
	"driver.tracking.lost":           {},
	"driver.tracking.restored":       {},
	"driver.exception_reported":      {},
	"driver.shipment_event_recorded": {},
}

func ParseDriverEventEnvelope(payload []byte) (DriverDomainEventEnvelope, *PermanentError) {
	var env DriverDomainEventEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return DriverDomainEventEnvelope{}, &PermanentError{Code: DriverEventErrorInvalidJSON}
	}
	if env.SchemaVersion != 0 && env.SchemaVersion != 1 {
		return DriverDomainEventEnvelope{}, &PermanentError{Code: DriverEventErrorUnsupportedVersion}
	}
	if strings.TrimSpace(env.EventID) == "" {
		return DriverDomainEventEnvelope{}, &PermanentError{Code: DriverEventErrorMissingEventID}
	}
	if strings.TrimSpace(env.TenantID) == "" {
		return DriverDomainEventEnvelope{}, &PermanentError{Code: DriverEventErrorMissingTenant}
	}
	if strings.TrimSpace(env.ShipmentID) == "" {
		return DriverDomainEventEnvelope{}, &PermanentError{Code: DriverEventErrorMissingShipment}
	}
	eventType := normalizeLegacyDriverEventType(strings.TrimSpace(env.EventType))
	if _, ok := allowedIngestDriverEventTypes[eventType]; !ok {
		return DriverDomainEventEnvelope{}, &PermanentError{Code: DriverEventErrorUnknownEventType}
	}
	env.EventType = eventType
	if env.OccurredAt.IsZero() {
		env.OccurredAt = time.Now().UTC()
	}
	return env, nil
}

func normalizeLegacyDriverEventType(eventType string) string {
	if eventType == "driver.exception_reported" {
		return "driver.problem.reported"
	}
	return eventType
}

func NormalizeDriverEvent(env DriverDomainEventEnvelope) (ControlTowerEvent, error) {
	eventID, err := uuid.Parse(strings.TrimSpace(env.EventID))
	if err != nil {
		return ControlTowerEvent{}, err
	}
	tenantID, err := uuid.Parse(strings.TrimSpace(env.TenantID))
	if err != nil {
		return ControlTowerEvent{}, err
	}
	shipmentID, err := uuid.Parse(strings.TrimSpace(env.ShipmentID))
	if err != nil {
		return ControlTowerEvent{}, err
	}
	subjectID := strings.TrimSpace(env.DriverID)
	if subjectID == "" {
		subjectID = shipmentID.String()
	}
	attrs := map[string]any{
		"eventType": env.EventType,
		"source":    firstNonEmptyString(env.Source, ControlTowerEventSourceDriver),
	}
	if env.ReasonCode != "" {
		attrs["reasonCode"] = env.ReasonCode
	}
	if env.ReasonText != "" {
		attrs["reasonText"] = env.ReasonText
	}
	if env.Latitude != nil && env.Longitude != nil {
		attrs["latitude"] = *env.Latitude
		attrs["longitude"] = *env.Longitude
	}
	if env.Accuracy != nil {
		attrs["accuracy"] = *env.Accuracy
	}
	if env.ETA != nil {
		attrs["eta"] = env.ETA.UTC().Format(time.RFC3339)
	}
	if env.DocumentID != "" {
		attrs["documentId"] = env.DocumentID
	}
	if len(env.Metadata) > 0 {
		attrs["metadata"] = env.Metadata
	}
	return ControlTowerEvent{
		ID: eventID, TenantID: tenantID, Type: env.EventType,
		Source: firstNonEmptyString(env.Source, ControlTowerEventSourceDriver),
		SubjectType: "driver", SubjectID: subjectID, ShipmentID: shipmentID,
		OccurredAt: env.OccurredAt.UTC(), Severity: env.Severity,
		Actor: firstNonEmptyString(env.ActorID, env.DriverID), Attributes: attrs,
	}, nil
}

func DriverEventPayloadSHA256(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func MapDriverAutomationTrigger(event ControlTowerEvent, env DriverDomainEventEnvelope) (AutomationTrigger, bool) {
	shipmentID := event.ShipmentID
	correlationID := ""
	if env.CorrelationID != nil {
		correlationID = *env.CorrelationID
	}
	if correlationID == "" {
		correlationID = "driver-event:" + event.ID.String()
	}
	baseAttrs := TriggerAttributes{
		EventType: event.Type, Source: event.Source, ReasonCode: env.ReasonCode,
		Severity: event.Severity, StateVersion: event.ID.String(),
	}
	switch event.Type {
	case "driver.delay.reported":
		return AutomationTrigger{
			TenantID: event.TenantID, TriggerType: DriverTriggerDelayReported,
			TriggerID: "driver-delay:" + event.ID.String(), ShipmentID: &shipmentID,
			WorkItemType: "driver_event", WorkItemID: event.ID.String(),
			CorrelationID: correlationID, OccurredAt: event.OccurredAt, Attributes: baseAttrs,
			SourceOrigin: "driver.delay.reported",
		}, true
	case "driver.problem.reported":
		return AutomationTrigger{
			TenantID: event.TenantID, TriggerType: DriverTriggerProblemReported,
			TriggerID: "driver-problem:" + event.ID.String(), ShipmentID: &shipmentID,
			ExceptionID: strings.ReplaceAll(event.ID.String(), "-", ""),
			WorkItemType: "driver_event", WorkItemID: event.ID.String(),
			CorrelationID: correlationID, OccurredAt: event.OccurredAt,
			Attributes: TriggerAttributes{
				EventType: event.Type, Source: event.Source, ReasonCode: env.ReasonCode,
				Severity: firstNonEmptyString(event.Severity, "high"), StateVersion: event.ID.String(),
				ExceptionCategory: strings.ToLower(strings.TrimSpace(env.ReasonCode)),
				Priority:          firstNonEmptyString(event.Severity, "high"),
			},
			SourceOrigin: "driver.problem.reported",
		}, true
	case "driver.tracking.lost":
		return AutomationTrigger{
			TenantID: event.TenantID, TriggerType: "tracking_lost",
			TriggerID: "tracking-lost:" + event.ID.String(), ShipmentID: &shipmentID,
			WorkItemType: "tracking", WorkItemID: shipmentID.String(),
			CorrelationID: correlationID, OccurredAt: event.OccurredAt,
			Attributes: TriggerAttributes{
				EventType: event.Type, TrackingStatus: "lost", Source: event.Source, StateVersion: event.ID.String(),
			},
			SourceOrigin: "driver.tracking.lost",
		}, true
	case "driver.tracking.restored":
		return AutomationTrigger{
			TenantID: event.TenantID, TriggerType: DriverTriggerTrackingRestored,
			TriggerID: "tracking-restored:" + event.ID.String(), ShipmentID: &shipmentID,
			WorkItemType: "tracking", WorkItemID: shipmentID.String(),
			CorrelationID: correlationID, OccurredAt: event.OccurredAt,
			Attributes: TriggerAttributes{
				EventType: event.Type, TrackingStatus: "active", Source: event.Source, StateVersion: event.ID.String(),
			},
			SourceOrigin: "driver.tracking.restored",
		}, true
	default:
		return AutomationTrigger{}, false
	}
}

func BuildDriverProblemExceptionSeed(event ControlTowerEvent, env DriverDomainEventEnvelope) EnsureExceptionSeed {
	return EnsureExceptionSeed{
		EventID: strings.ReplaceAll(event.ID.String(), "-", ""),
		ShipmentID: event.ShipmentID.String(),
		EventType: strings.ToLower(strings.TrimSpace(env.ReasonCode)),
		Source: "driver",
		Severity: firstNonEmptyString(event.Severity, "high"),
		OccurredAt: event.OccurredAt,
	}
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
