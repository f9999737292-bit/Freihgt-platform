package controltower

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"time"
)

var criticalEventIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func ValidateCriticalEventID(eventID string) bool {
	return criticalEventIDPattern.MatchString(eventID)
}

func FindCriticalEventByID(events []ControlTowerEvent, eventID string) (ControlTowerEvent, bool) {
	for _, event := range events {
		if event.ID == eventID {
			return event, true
		}
	}
	return ControlTowerEvent{}, false
}

func BuildCriticalEvents(
	shipments []ControlTowerShipment,
	shipmentIDsWithDocs map[string]struct{},
	thresholds SLAThresholds,
	now time.Time,
) []ControlTowerEvent {
	events := make([]ControlTowerEvent, 0)

	for _, shipment := range shipments {
		if !IsActiveShipmentStatus(shipment.Status) && shipment.Status != "CANCELLED" {
			continue
		}

		if shipment.Status == "CANCELLED" {
			occurredAt := pickTime(shipment.LastUpdatedAt, now)
			events = append(events, ControlTowerEvent{
				ID:             deterministicEventID(shipment.ID, EventTypeShipmentCancelled, canonicalEventAnchor(shipment, EventTypeShipmentCancelled)),
				ShipmentID:     shipment.ID,
				ShipmentNumber: shipment.ShipmentNumber,
				Type:           EventTypeShipmentCancelled,
				Severity:       EventSeverityCritical,
				OccurredAt:     occurredAt,
				Source:         EventSourceControlTower,
			})
			continue
		}

		if shipment.SLAReason != nil && *shipment.SLAReason == SLAReasonPickupOverdue && shipment.PlannedPickupAt != nil {
			events = append(events, ControlTowerEvent{
				ID:             deterministicEventID(shipment.ID, EventTypePickupDelay, canonicalEventAnchor(shipment, EventTypePickupDelay)),
				ShipmentID:     shipment.ID,
				ShipmentNumber: shipment.ShipmentNumber,
				Type:           EventTypePickupDelay,
				Severity:       severityForSLA(shipment.SLAStatus),
				OccurredAt:     *shipment.PlannedPickupAt,
				Source:         EventSourceControlTower,
			})
		}

		if shipment.SLAReason != nil && *shipment.SLAReason == SLAReasonDeliveryOverdue && shipment.PlannedDeliveryAt != nil {
			events = append(events, ControlTowerEvent{
				ID:             deterministicEventID(shipment.ID, EventTypeDeliveryDelay, canonicalEventAnchor(shipment, EventTypeDeliveryDelay)),
				ShipmentID:     shipment.ID,
				ShipmentNumber: shipment.ShipmentNumber,
				Type:           EventTypeDeliveryDelay,
				Severity:       severityForSLA(shipment.SLAStatus),
				OccurredAt:     *shipment.PlannedDeliveryAt,
				Source:         EventSourceControlTower,
			})
		}

		if shipment.SLAReason != nil && *shipment.SLAReason == SLAReasonStaleUpdates && shipment.LastUpdatedAt != nil {
			events = append(events, ControlTowerEvent{
				ID:             deterministicEventID(shipment.ID, EventTypeStaleUpdates, canonicalEventAnchor(shipment, EventTypeStaleUpdates)),
				ShipmentID:     shipment.ID,
				ShipmentNumber: shipment.ShipmentNumber,
				Type:           EventTypeStaleUpdates,
				Severity:       severityForSLA(shipment.SLAStatus),
				OccurredAt:     *shipment.LastUpdatedAt,
				Source:         EventSourceControlTower,
			})
		}

		if IsDeliveredShipmentStatus(shipment.Status) {
			if _, ok := shipmentIDsWithDocs[shipment.ID]; !ok {
				occurredAt := pickTime(shipment.LastUpdatedAt, now)
				events = append(events, ControlTowerEvent{
					ID:             deterministicEventID(shipment.ID, EventTypeMissingDocuments, canonicalEventAnchor(shipment, EventTypeMissingDocuments)),
					ShipmentID:     shipment.ID,
					ShipmentNumber: shipment.ShipmentNumber,
					Type:           EventTypeMissingDocuments,
					Severity:       EventSeverityWarning,
					OccurredAt:     occurredAt,
					Source:         EventSourceControlTower,
				})
			}
		}

		if shipment.SLAStatus == SLAStatusCritical && shipment.SLAReason != nil && *shipment.SLAReason == SLAReasonTechnicalProblem {
			occurredAt := pickTime(shipment.LastUpdatedAt, now)
			events = append(events, ControlTowerEvent{
				ID:             deterministicEventID(shipment.ID, EventTypeTechnicalProblem, canonicalEventAnchor(shipment, EventTypeTechnicalProblem)),
				ShipmentID:     shipment.ID,
				ShipmentNumber: shipment.ShipmentNumber,
				Type:           EventTypeTechnicalProblem,
				Severity:       EventSeverityCritical,
				OccurredAt:     occurredAt,
				Source:         EventSourceControlTower,
			})
		}
	}

	sortEventsByOccurredAtDesc(events)
	return events
}

func canonicalEventAnchor(shipment ControlTowerShipment, eventType string) time.Time {
	switch eventType {
	case EventTypePickupDelay:
		if shipment.PlannedPickupAt != nil {
			return shipment.PlannedPickupAt.UTC()
		}
	case EventTypeDeliveryDelay:
		if shipment.PlannedDeliveryAt != nil {
			return shipment.PlannedDeliveryAt.UTC()
		}
	case EventTypeShipmentCancelled, EventTypeStaleUpdates, EventTypeMissingDocuments, EventTypeTechnicalProblem:
		return time.Unix(0, 0).UTC()
	}
	return pickTime(shipment.LastUpdatedAt, time.Now().UTC())
}

func severityForSLA(status SLAStatus) string {
	switch status {
	case SLAStatusCritical:
		return EventSeverityCritical
	case SLAStatusDelayed, SLAStatusAtRisk:
		return EventSeverityWarning
	default:
		return EventSeverityInfo
	}
}

func deterministicEventID(shipmentID, eventType string, anchor time.Time) string {
	raw := fmt.Sprintf("%s:%s:%d", shipmentID, eventType, anchor.UTC().Unix())
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:16])
}

func pickTime(value *time.Time, fallback time.Time) time.Time {
	if value != nil {
		return value.UTC()
	}
	return fallback.UTC()
}

func sortEventsByOccurredAtDesc(events []ControlTowerEvent) {
	for i := 0; i < len(events); i++ {
		for j := i + 1; j < len(events); j++ {
			if events[j].OccurredAt.After(events[i].OccurredAt) {
				events[i], events[j] = events[j], events[i]
			}
		}
	}
}
