package shipmentevents

import (
	"time"

	"github.com/freight-platform/api-gateway/internal/platform/sla"
)

func buildDerivedShipmentEvents(shipment rawShipment) []ShipmentTimelineEvent {
	events := make([]ShipmentTimelineEvent, 0, 6)

	if shipment.CreatedAt != nil {
		events = append(events, newDerivedEvent(shipment, EventTypeShipmentCreated, CategoryShipment, SourceShipmentState, *shipment.CreatedAt, "", nil))
	}
	if shipment.PlannedPickupAt != nil {
		events = append(events, newDerivedEvent(shipment, EventTypePickupPlanned, CategoryOperation, SourceShipmentState, *shipment.PlannedPickupAt, "", nil))
	}
	if shipment.ActualPickupAt != nil {
		events = append(events, newDerivedEvent(shipment, EventTypePickupCompleted, CategoryOperation, SourceShipmentState, *shipment.ActualPickupAt, "", nil))
	}
	if shipment.PlannedDeliveryAt != nil {
		events = append(events, newDerivedEvent(shipment, EventTypeDeliveryPlanned, CategoryOperation, SourceShipmentState, *shipment.PlannedDeliveryAt, "", nil))
	}
	if shipment.ActualDeliveryAt != nil {
		events = append(events, newDerivedEvent(shipment, EventTypeDeliveryCompleted, CategoryOperation, SourceShipmentState, *shipment.ActualDeliveryAt, "", nil))
	}

	return events
}

func buildDocumentEvents(shipment rawShipment, documents []rawDocument) []ShipmentTimelineEvent {
	events := make([]ShipmentTimelineEvent, 0, len(documents)*3)
	for _, doc := range documents {
		if doc.CreatedAt != nil {
			desc := descriptionCode(EventTypeDocumentCreated)
			events = append(events, ShipmentTimelineEvent{
				ID:              deterministicEventID(shipment.ID, EventTypeDocumentCreated, *doc.CreatedAt, doc.ID, SourceDocumentState),
				ShipmentID:      shipment.ID,
				ShipmentNumber:  shipment.ShipmentNumber,
				Type:            EventTypeDocumentCreated,
				Category:        CategoryDocument,
				Source:          SourceDocumentState,
				Severity:        SeverityInfo,
				TitleCode:       titleCode(EventTypeDocumentCreated),
				DescriptionCode: &desc,
				OccurredAt:      doc.CreatedAt.UTC(),
				RecordedAt:      doc.CreatedAt,
				Metadata:        documentMetadata(doc),
				Derived:         true,
			})
		}

		if doc.SignedAt != nil {
			desc := descriptionCode(EventTypeDocumentSigned)
			events = append(events, ShipmentTimelineEvent{
				ID:              deterministicEventID(shipment.ID, EventTypeDocumentSigned, *doc.SignedAt, doc.ID, SourceDocumentState),
				ShipmentID:      shipment.ID,
				ShipmentNumber:  shipment.ShipmentNumber,
				Type:            EventTypeDocumentSigned,
				Category:        CategoryDocument,
				Source:          SourceDocumentState,
				Severity:        SeverityInfo,
				TitleCode:       titleCode(EventTypeDocumentSigned),
				DescriptionCode: &desc,
				OccurredAt:      doc.SignedAt.UTC(),
				RecordedAt:      doc.SignedAt,
				Metadata:        documentMetadata(doc),
				Derived:         true,
			})
		}

		if doc.RejectedAt != nil {
			desc := descriptionCode(EventTypeDocumentRejected)
			events = append(events, ShipmentTimelineEvent{
				ID:              deterministicEventID(shipment.ID, EventTypeDocumentRejected, *doc.RejectedAt, doc.ID, SourceDocumentState),
				ShipmentID:      shipment.ID,
				ShipmentNumber:  shipment.ShipmentNumber,
				Type:            EventTypeDocumentRejected,
				Category:        CategoryDocument,
				Source:          SourceDocumentState,
				Severity:        SeverityWarning,
				TitleCode:       titleCode(EventTypeDocumentRejected),
				DescriptionCode: &desc,
				OccurredAt:      doc.RejectedAt.UTC(),
				RecordedAt:      doc.RejectedAt,
				Metadata:        documentMetadata(doc),
				Derived:         true,
			})
		}
	}
	return events
}

func buildSLAEvent(shipment rawShipment, thresholds sla.Thresholds, now time.Time) *ShipmentTimelineEvent {
	result := sla.Compute(sla.Input{
		Status:            shipment.Status,
		PlannedPickupAt:   shipment.PlannedPickupAt,
		PlannedDeliveryAt: shipment.PlannedDeliveryAt,
		ActualPickupAt:    shipment.ActualPickupAt,
		ActualDeliveryAt:  shipment.ActualDeliveryAt,
		LastUpdatedAt:     shipment.UpdatedAt,
		TechnicalProblem:  shipment.TechnicalProblem,
		Now:               now,
		Thresholds:        thresholds,
	})

	var eventType string
	switch result.Status {
	case sla.StatusAtRisk:
		eventType = EventTypeSLAAtRisk
	case sla.StatusDelayed:
		eventType = EventTypeSLADelayed
	case sla.StatusCritical:
		if result.Reason == sla.ReasonTechnicalProblem {
			return nil
		}
		if result.Reason == sla.ReasonCancelled {
			return nil
		}
		eventType = EventTypeSLACritical
	default:
		return nil
	}

	var occurredAt *time.Time
	switch result.Reason {
	case sla.ReasonDeliveryOverdue, sla.ReasonDeliveryAtRisk:
		occurredAt = shipment.PlannedDeliveryAt
	case sla.ReasonPickupOverdue, sla.ReasonPickupAtRisk:
		occurredAt = shipment.PlannedPickupAt
	case sla.ReasonStaleUpdates:
		occurredAt = shipment.UpdatedAt
	case sla.ReasonCompletedLate:
		occurredAt = shipment.ActualDeliveryAt
	default:
		return nil
	}
	if occurredAt == nil {
		return nil
	}

	meta := sanitizeMetadata(map[string]interface{}{
		"slaStatus": string(result.Status),
		"slaReason": result.Reason,
	})
	if result.DelayMinutes != nil {
		meta = sanitizeMetadata(map[string]interface{}{
			"slaStatus":    string(result.Status),
			"slaReason":    result.Reason,
			"delayMinutes": *result.DelayMinutes,
		})
	}

	event := newDerivedEvent(shipment, eventType, CategorySLA, SourceSLACalculator, *occurredAt, "", meta)
	return &event
}

func newDerivedEvent(shipment rawShipment, eventType, category, source string, occurredAt time.Time, relatedEntityID string, metadata map[string]interface{}) ShipmentTimelineEvent {
	desc := descriptionCode(eventType)
	return ShipmentTimelineEvent{
		ID:              deterministicEventID(shipment.ID, eventType, occurredAt, relatedEntityID, source),
		ShipmentID:      shipment.ID,
		ShipmentNumber:  shipment.ShipmentNumber,
		Type:            eventType,
		Category:        category,
		Source:          source,
		Severity:        severityForType(eventType),
		TitleCode:       titleCode(eventType),
		DescriptionCode: &desc,
		OccurredAt:      occurredAt.UTC(),
		Metadata:        sanitizeMetadata(metadata),
		Derived:         true,
	}
}
